#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[derive(Clone, Debug)]
pub struct File<T>
where
    T: std::hash::Hasher + Clone,
{
    base: std::path::PathBuf,
    hasher: T,
}

impl<T> File<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    pub fn new(base: std::path::PathBuf, hasher: T) -> Self {
        Self { base, hasher }
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::DocumentStorage for File<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save(&self, document: &crate::Document) -> Result<u64, error::Error> {
        let file = serde_binary::ser::to_vec(&document)?;
        let mut hasher = self.hasher.clone();
        hasher.write(&file);
        let hash = hasher.finish();
        let path = self.base.join(format!("{hash:x}"));
        if !std::path::Path::exists(&path) {
            let content = serde_binary::ser::to_vec(&document)?;
            std::fs::write(path, content)?;
        }
        Ok(hash)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn find(&self, uuid: u64) -> Result<crate::Document, error::Error> {
        let path = self.base.join(format!("{uuid:x}"));
        let v = std::fs::read(path)?;
        let document: crate::Document = serde_binary::de::from_slice(&v)?;
        Ok(document)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn count(&self) -> Result<i64, error::Error> {
        Ok(std::fs::read_dir(&self.base)?
            .filter(|r| {
                if let Ok(entry) = r {
                    entry.path().is_file()
                } else {
                    false
                }
            })
            .count() as i64)
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::TokenStorage for File<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error> {
        let mut hasher = self.hasher.clone();
        hasher.write(index.token.as_bytes());
        let hash = hasher.finish();
        let path = self.base.join(format!("{hash:x}"));
        if std::path::Path::exists(&path) {
            let v = std::fs::read(&path)?;
            let old_postings_list = crate::PostingsList::from_bytes(v);
            let new_postings_list = old_postings_list.union(index.postings_list.clone());
            if old_postings_list != new_postings_list {
                std::fs::write(path, new_postings_list.as_bytes())?;
            }
        } else {
            std::fs::write(path, index.postings_list.as_bytes())?;
        }
        Ok(())
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
        let hash = format!("{:x}", hasher.finish());
        let path = self.base.join(hash);
        let postings_list = std::fs::read(path)?;
        Ok(crate::PostingsList::from_bytes(postings_list))
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
        for index in indices {
            let mut hasher = self.hasher.clone();
            hasher.write(index.token.as_bytes());
            let hash = hasher.finish();
            let path = self.base.join(format!("{hash:x}"));
            if std::path::Path::exists(&path) {
                let v = std::fs::read(&path)?;
                let old_postings_list = crate::PostingsList::from_bytes(v);
                let new_postings_list = old_postings_list.union(index.postings_list.clone());
                if old_postings_list != new_postings_list {
                    std::fs::write(path, new_postings_list.as_bytes())?;
                }
            } else {
                std::fs::write(path, index.postings_list.as_bytes())?;
            }
        }
        Ok(())
    }
}
