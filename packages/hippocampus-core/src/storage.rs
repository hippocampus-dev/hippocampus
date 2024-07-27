#[cfg(test)]
use mockall::{automock, predicate::*};

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
}
