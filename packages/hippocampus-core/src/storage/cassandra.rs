use cdrs_tokio::cluster::session::SessionBuilder;
use cdrs_tokio::types::IntoRustByName;
use cdrs_tokio::types::prelude::Blob;
#[cfg(debug_assertions)]
use elapsed::prelude::*;

const MAX_CAS_RETRIES: usize = 10;

type CurrentSession = cdrs_tokio::cluster::session::Session<
    cdrs_tokio::transport::TransportTcp,
    cdrs_tokio::cluster::TcpConnectionManager,
    cdrs_tokio::load_balancing::RoundRobinLoadBalancingStrategy<
        cdrs_tokio::transport::TransportTcp,
        cdrs_tokio::cluster::TcpConnectionManager,
    >,
>;

#[derive(Clone, Debug)]
pub struct Cassandra<T>
where
    T: std::hash::Hasher + Clone,
{
    connection: std::sync::Arc<CurrentSession>,
    hasher: T,
}

impl<T> Cassandra<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    pub async fn new<S>(address: S, hasher: T) -> Result<Self, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug + Send + Sync,
    {
        let config = cdrs_tokio::cluster::NodeTcpConfigBuilder::new()
            .with_contact_point(address.as_ref().into())
            .build()
            .await?;
        let connection = cdrs_tokio::cluster::session::TcpSessionBuilder::new(
            cdrs_tokio::load_balancing::RoundRobinLoadBalancingStrategy::new(),
            config,
        )
        .build()
        .await?;
        connection.query("CREATE KEYSPACE IF NOT EXISTS hippocampus WITH REPLICATION = { 'class': 'SimpleStrategy', 'replication_factor': 1 };").await?;
        connection.query("CREATE TABLE IF NOT EXISTS hippocampus.tokens (body text PRIMARY KEY, postings_list blob);").await?;
        Ok(Self {
            connection: std::sync::Arc::new(connection),
            hasher,
        })
    }
}

#[derive(Debug)]
pub enum Error {
    NotFound,
    CasRetryExceeded,
}

impl std::error::Error for Error {}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::NotFound => {
                write!(f, "not found")
            }
            Error::CasRetryExceeded => {
                write!(f, "CAS retry exceeded")
            }
        }
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::TokenStorage for Cassandra<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error> {
        let postings_list: Blob = index.postings_list.as_bytes().into();

        let result = self
            .connection
            .query_with_values(
                "INSERT INTO hippocampus.tokens (body, postings_list) VALUES (?, ?) IF NOT EXISTS;",
                cdrs_tokio::query_values!("body" => index.token.clone(), "postings_list" => postings_list),
            )
            .await?
            .response_body()?;

        if let Some(row) = result.into_rows().and_then(|r| r.into_iter().next()) {
            let applied: bool = row.get_r_by_name("[applied]")?;
            if applied {
                return Ok(());
            }
        }

        for _ in 0..MAX_CAS_RETRIES {
            if let Some(row) = self
                .connection
                .query_with_values(
                    "SELECT postings_list FROM hippocampus.tokens WHERE body = ?;",
                    cdrs_tokio::query_values!("body" => index.token.clone()),
                )
                .await?
                .response_body()?
                .into_rows()
                // body is primary key, so there should be only one row
                .and_then(|r| r.into_iter().next())
            {
                let postings_list: Blob = row.get_r_by_name("postings_list")?;
                let old_postings_list =
                    crate::PostingsList::from_bytes(postings_list.clone().into_vec());
                let new_postings_list = old_postings_list.union(index.postings_list.clone());

                if old_postings_list == new_postings_list {
                    return Ok(());
                }

                let result = self
                    .connection
                    .query_with_values(
                        "UPDATE hippocampus.tokens SET postings_list = ? WHERE body = ? IF postings_list = ?;",
                        cdrs_tokio::query_values!(
                            "body" => index.token.clone(),
                            "postings_list" => Blob::from(new_postings_list.as_bytes()),
                            "old_postings_list" => postings_list
                        ),
                    )
                    .await?
                    .response_body()?;

                if let Some(row) = result.into_rows().and_then(|r| r.into_iter().next()) {
                    let applied: bool = row.get_r_by_name("[applied]")?;
                    if applied {
                        return Ok(());
                    }
                }
            }
        }

        Err(error::Error::from(Error::CasRetryExceeded))
    }

    #[cfg(not(test))]
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn get_postings_list<S: AsRef<str> + std::fmt::Debug + Send + Sync>(
        &self,
        token: S,
    ) -> Result<crate::PostingsList, error::Error> {
        if let Some(row) = self
            .connection
            .query_with_values(
                "SELECT postings_list FROM hippocampus.tokens WHERE body = ?;",
                cdrs_tokio::query_values!("body" => token.as_ref()),
            )
            .await?
            .response_body()?
            .into_rows()
            // body is primary key, so there should be only one row
            .and_then(|row| row.into_iter().next())
        {
            let postings_list: Blob = row.get_r_by_name("postings_list")?;
            return Ok(crate::PostingsList::from_bytes(postings_list.into_vec()));
        }
        Err(error::Error::from(Error::NotFound))
    }

    #[cfg(test)]
    async fn get_postings_list(&self, _token: &str) -> Result<crate::PostingsList, error::Error> {
        unimplemented!()
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_postings_list_bulk(
        &self,
        indices: Vec<crate::InvertedIndex>,
    ) -> Result<(), error::Error> {
        let futures: Vec<_> = indices
            .into_iter()
            .map(|index| {
                let this = self.clone();
                async move { this.save_postings_list(index).await }
            })
            .collect();
        futures::future::try_join_all(futures).await?;
        Ok(())
    }
}
