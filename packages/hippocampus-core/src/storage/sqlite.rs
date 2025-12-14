use opentelemetry::trace::TraceContextExt;
use tracing_opentelemetry::OpenTelemetrySpanExt;

#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[derive(Clone, Debug)]
pub struct SQLite<T>
where
    T: std::hash::Hasher + Clone,
{
    connection: std::sync::Arc<std::sync::Mutex<rusqlite::Connection>>,
    hasher: T,
}

impl<T> SQLite<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    pub fn new(path: Option<std::path::PathBuf>, hasher: T) -> Result<Self, error::Error> {
        let connection = if let Some(path) = path {
            rusqlite::Connection::open(path)?
        } else {
            rusqlite::Connection::open_in_memory()?
        };

        connection.execute(
            &sqlcommenter("CREATE TABLE IF NOT EXISTS documents (id INTEGER PRIMARY KEY, body BLOB NOT NULL);"),
            [],
        )?;
        connection.execute(
            &sqlcommenter("CREATE TABLE IF NOT EXISTS tokens (body PRIMARY KEY, postings_list BLOB NOT NULL);"),
            [],
        )?;
        Ok(Self {
            connection: std::sync::Arc::new(std::sync::Mutex::new(connection)),
            hasher,
        })
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::DocumentStorage for SQLite<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save(&self, document: &crate::Document) -> Result<u64, error::Error> {
        let mut locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let tx = locked_connection.transaction()?;
        let content = serde_binary::ser::to_vec(&document)?;
        tx.execute(
            &sqlcommenter("INSERT OR IGNORE INTO documents (body) VALUES (?);"),
            rusqlite::params![content],
        )?;
        let id = if tx.changes() == 0 {
            let mut statement =
                tx.prepare(&sqlcommenter("SELECT id FROM documents WHERE body = ?;"))?;
            statement.query_row([&content], |row| row.get::<usize, i64>(0))?
        } else {
            tx.last_insert_rowid()
        };
        tx.commit()?;
        Ok(id as u64)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn find(&self, uuid: u64) -> Result<crate::Document, error::Error> {
        let locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let mut statement =
            locked_connection.prepare(&sqlcommenter("SELECT body FROM documents where id = ?;"))?;
        let v = statement.query_row([uuid], |row| row.get::<usize, Vec<u8>>(0))?;
        let document: crate::Document = serde_binary::de::from_slice(&v)?;
        Ok(document)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn count(&self) -> Result<i64, error::Error> {
        let locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let mut statement =
            locked_connection.prepare(&sqlcommenter("SELECT COUNT(*) FROM documents;"))?;
        let count = statement.query_row([], |row| row.get::<usize, i64>(0))?;
        Ok(count)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_bulk(&self, documents: &[crate::Document]) -> Result<Vec<u64>, error::Error> {
        let mut locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let tx = locked_connection.transaction()?;
        let mut v = Vec::with_capacity(documents.len());
        {
            let mut statement =
                tx.prepare(&sqlcommenter("SELECT id FROM documents WHERE body = ?;"))?;
            for document in documents {
                let content = serde_binary::ser::to_vec(document)?;
                tx.execute(
                    &sqlcommenter("INSERT OR IGNORE INTO documents (body) VALUES (?);"),
                    rusqlite::params![content],
                )?;
                let id = if tx.changes() == 0 {
                    statement.query_row([&content], |row| row.get::<usize, i64>(0))?
                } else {
                    tx.last_insert_rowid()
                };
                v.push(id as u64);
            }
        }
        tx.commit()?;
        Ok(v)
    }
}

#[async_trait::async_trait]
impl<T> crate::storage::TokenStorage for SQLite<T>
where
    T: std::hash::Hasher + Clone + Send + Sync,
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error> {
        let mut locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let tx = locked_connection.transaction()?;
        {
            let mut statement = tx.prepare(&sqlcommenter(
                "SELECT postings_list FROM tokens WHERE body = ?;",
            ))?;
            if let Ok(postings_list) = statement.query_row([index.token.clone()], |row| {
                let postings_list = row.get::<usize, Vec<u8>>(0)?;
                Ok(postings_list)
            }) {
                let old_postings_list = crate::PostingsList::from_bytes(postings_list);
                let new_postings_list = old_postings_list.union(index.postings_list.clone());
                if old_postings_list != new_postings_list {
                    tx.execute(
                        &sqlcommenter("UPDATE tokens SET postings_list = ? WHERE body = ?;"),
                        rusqlite::params![new_postings_list.as_bytes(), index.token],
                    )?;
                }
            } else {
                tx.execute(
                    &sqlcommenter("INSERT INTO tokens (body, postings_list) VALUES (?, ?);"),
                    rusqlite::params![index.token, index.postings_list.as_bytes()],
                )?;
            }
        }
        tx.commit()?;
        Ok(())
    }

    #[cfg(not(test))]
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn get_postings_list<S: AsRef<str> + std::fmt::Debug + Send + Sync>(
        &self,
        token: S,
    ) -> Result<crate::PostingsList, error::Error> {
        let locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let mut statement = locked_connection.prepare(&sqlcommenter(
            "SELECT postings_list FROM tokens WHERE body = ?;",
        ))?;
        let postings_list =
            statement.query_row([token.as_ref()], |row| row.get::<usize, Vec<u8>>(0))?;
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
        let mut locked_connection = self
            .connection
            .lock()
            .map_err(|e| error::Error::from_message(e.to_string()))?;
        let tx = locked_connection.transaction()?;
        {
            let mut statement = tx.prepare(&sqlcommenter(
                "SELECT postings_list FROM tokens WHERE body = ?;",
            ))?;
            for index in indices {
                if let Ok(postings_list) = statement.query_row([index.token.clone()], |row| {
                    let postings_list = row.get::<usize, Vec<u8>>(0)?;
                    Ok(postings_list)
                }) {
                    let old_postings_list = crate::PostingsList::from_bytes(postings_list);
                    let new_postings_list = old_postings_list.union(index.postings_list.clone());
                    if old_postings_list != new_postings_list {
                        tx.execute(
                            &sqlcommenter("UPDATE tokens SET postings_list = ? WHERE body = ?;"),
                            rusqlite::params![new_postings_list.as_bytes(), index.token],
                        )?;
                    }
                } else {
                    tx.execute(
                        &sqlcommenter("INSERT INTO tokens (body, postings_list) VALUES (?, ?);"),
                        rusqlite::params![index.token, index.postings_list.as_bytes()],
                    )?;
                }
            }
        }
        tx.commit()?;
        Ok(())
    }
}

fn sqlcommenter<S>(s: S) -> String
where
    S: AsRef<str>,
{
    let context = tracing::Span::current().context();
    let span = context.span();
    let span_context = span.span_context();
    let trace_id = span_context.trace_id();
    let span_id = span_context.span_id();
    let trace_flags = span_context.trace_flags();

    format!(
        "{} /*traceid='{}',spanid='{}',traceflags='{}'*/",
        s.as_ref(),
        trace_id,
        span_id,
        trace_flags.to_u8(),
    )
}
