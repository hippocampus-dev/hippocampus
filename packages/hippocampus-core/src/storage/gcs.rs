#[cfg(debug_assertions)]
use elapsed::prelude::*;
#[cfg(not(test))]
use error::context::Context;

const MAX_CAS_RETRIES: usize = 10;

#[derive(Debug)]
pub enum Error {
    CasRetryExceeded,
}

impl std::error::Error for Error {}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::CasRetryExceeded => {
                write!(f, "CAS retry exceeded")
            }
        }
    }
}

#[derive(Clone, Debug)]
pub struct GCS<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    client: gcs::Client,
    bucket: String,
    prefix: String,
    hasher: T,
}

impl<T> GCS<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    pub fn new(client: gcs::Client, bucket: String, prefix: String, hasher: T) -> Self {
        Self {
            client,
            bucket,
            prefix,
            hasher,
        }
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::TokenStorage for GCS<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error> {
        let mut hasher = self.hasher.clone();
        hasher.write(index.token.as_bytes());
        let hash = hasher.finish();
        let key = format!("{}{:x}", self.prefix, hash);

        match self
            .client
            .upload(&self.bucket, &key, &index.postings_list.as_bytes(), Some(0))
            .await
        {
            Ok(_) => return Ok(()),
            Err(e) => {
                if !e
                    .downcast_ref::<retry::Error>()
                    .and_then(|e| e.downcast_ref::<gcs::Error>())
                    .map(|e| matches!(e, gcs::Error::PreconditionFailed))
                    .unwrap_or(false)
                {
                    return Err(e);
                }
            }
        }

        for _ in 0..MAX_CAS_RETRIES {
            let (body, generation) = match self.client.download(&self.bucket, &key).await {
                Ok(result) => result,
                Err(e) => {
                    if e.downcast_ref::<retry::Error>()
                        .and_then(|e| e.downcast_ref::<gcs::Error>())
                        .map(|e| matches!(e, gcs::Error::NotFound))
                        .unwrap_or(false)
                    {
                        match self
                            .client
                            .upload(&self.bucket, &key, &index.postings_list.as_bytes(), Some(0))
                            .await
                        {
                            Ok(_) => return Ok(()),
                            Err(e) => {
                                if e.downcast_ref::<retry::Error>()
                                    .and_then(|e| e.downcast_ref::<gcs::Error>())
                                    .map(|e| matches!(e, gcs::Error::PreconditionFailed))
                                    .unwrap_or(false)
                                {
                                    continue;
                                }
                                return Err(e);
                            }
                        }
                    }
                    return Err(e);
                }
            };

            let old_postings_list = crate::PostingsList::from_bytes(body);
            let new_postings_list = old_postings_list.union(index.postings_list.clone());

            if old_postings_list == new_postings_list {
                return Ok(());
            }

            return match self
                .client
                .upload(
                    &self.bucket,
                    &key,
                    &new_postings_list.as_bytes(),
                    Some(generation),
                )
                .await
            {
                Ok(_) => Ok(()),
                Err(e) => {
                    if e.downcast_ref::<retry::Error>()
                        .and_then(|e| e.downcast_ref::<gcs::Error>())
                        .map(|e| matches!(e, gcs::Error::PreconditionFailed))
                        .unwrap_or(false)
                    {
                        continue;
                    }
                    Err(e)
                }
            };
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
        let mut hasher = self.hasher.clone();
        hasher.write(token.as_ref().as_bytes());
        let hash = hasher.finish();
        let key = format!("{}{:x}", self.prefix, hash);
        let (body, _) = self
            .client
            .download(&self.bucket, &key)
            .await
            .context("error occurred while downloading a token")?;
        Ok(crate::PostingsList::from_bytes(body))
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
