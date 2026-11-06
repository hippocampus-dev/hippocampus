#[cfg(test)]
use mockall::{automock, predicate::*};

pub mod cached;
#[cfg(feature = "cassandra")]
pub mod cassandra;
pub mod file;
pub mod gcs;
#[cfg(feature = "sqlite")]
pub mod sqlite;

#[async_trait::async_trait]
#[cfg_attr(test, automock)]
pub trait DocumentStorage {
    async fn save(&self, content: &crate::Document) -> Result<u64, error::Error>;
    async fn find(&self, uuid: u64) -> Result<crate::Document, error::Error>;
    async fn count(&self) -> Result<i64, error::Error>;

    async fn save_bulk(&self, documents: &[crate::Document]) -> Result<Vec<u64>, error::Error> {
        let mut v = Vec::with_capacity(documents.len());
        for document in documents {
            v.push(self.save(document).await?);
        }
        Ok(v)
    }
}

#[async_trait::async_trait]
#[cfg_attr(test, automock)]
pub trait TokenStorage {
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error>;
    #[cfg(not(test))]
    async fn get_postings_list<S: AsRef<str> + std::fmt::Debug + Send + Sync>(
        &self,
        token: S,
    ) -> Result<crate::PostingsList, error::Error>;
    #[cfg(test)] // automock has a problem with generic function
    async fn get_postings_list(&self, token: &str) -> Result<crate::PostingsList, error::Error>;

    async fn save_postings_list_bulk(
        &self,
        indices: Vec<crate::InvertedIndex>,
    ) -> Result<(), error::Error> {
        for index in indices {
            self.save_postings_list(index).await?;
        }
        Ok(())
    }
}
