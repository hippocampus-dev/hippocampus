#[cfg(debug_assertions)]
use elapsed::prelude::*;
use error::context::Context;

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
        match self.client.download(&self.bucket, &key).await {
            Ok(body) => {
                let old_postings_list = crate::PostingsList::from_bytes(body);
                let new_postings_list = old_postings_list.union(index.postings_list.clone());
                if old_postings_list != new_postings_list {
                    let _ = self
                        .client
                        .upload(&self.bucket, &key, &new_postings_list.as_bytes())
                        .await
                        .context("error occurred while uploading a token to update")?;
                }
                Ok(())
            }
            Err(e) => {
                if e.downcast_ref::<retry::Error>()
                    .and_then(|e| e.downcast_ref::<gcs::Error>())
                    .is_some()
                {
                    let _ = self
                        .client
                        .upload(&self.bucket, &key, &index.postings_list.as_bytes())
                        .await
                        .context("error occurred while uploading a token to create")?;
                    return Ok(());
                }
                Err(e)
            }
        }
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
        let postings_list = self
            .client
            .download(&self.bucket, &key)
            .await
            .context("error occurred while downloading a token")?;
        Ok(crate::PostingsList::from_bytes(postings_list))
    }

    #[cfg(test)]
    async fn get_postings_list(&self, _token: &str) -> Result<crate::PostingsList, error::Error> {
        unimplemented!()
    }
}
