//! Configuration parsing for hippocampus applications.
//!
//! # Feature Flags
//!
//! - `sqlite`: Enable SQLite storage backend
//! - `gcs`: Enable Google Cloud Storage backend
//! - `cassandra`: Enable Cassandra storage backend
//! - `wasm`: Enable WASM tokenizer support

#[derive(Clone, Debug, serde::Deserialize)]
pub struct Configuration {
    #[serde(rename = "TokenStorage")]
    pub token_storage: TokenStorageConfiguration,
    #[serde(rename = "DocumentStorage")]
    pub document_storage: DocumentStorageConfiguration,
    #[serde(rename = "Tokenizer")]
    pub tokenizer: TokenizerConfiguration,
    #[serde(rename = "Schema")]
    pub schema: SchemaConfiguration,
}

#[derive(Clone, Debug, serde::Deserialize)]
#[serde(tag = "kind")]
pub enum DocumentStorageConfiguration {
    File {
        path: std::path::PathBuf,
    },
    #[cfg(feature = "sqlite")]
    SQLite {
        path: std::path::PathBuf,
    },
}

#[derive(Clone, Debug, serde::Deserialize)]
#[serde(tag = "kind")]
#[allow(clippy::upper_case_acronyms)]
pub enum TokenStorageConfiguration {
    File {
        path: std::path::PathBuf,
    },
    #[cfg(feature = "sqlite")]
    SQLite {
        path: std::path::PathBuf,
    },
    #[cfg(feature = "gcs")]
    GCS {
        bucket: String,
        prefix: String,
        service_account_key_path: std::path::PathBuf,
    },
    #[cfg(feature = "cassandra")]
    Cassandra {
        address: String,
    },
}

#[derive(Clone, Debug, serde::Deserialize)]
#[serde(tag = "kind")]
pub enum TokenizerConfiguration {
    Lindera,
    Whitespace,
    #[cfg(feature = "wasm")]
    Wasm {
        path: std::path::PathBuf,
    },
}

#[derive(Clone, Debug, serde::Deserialize)]
pub struct SchemaConfiguration {
    pub fields: Vec<FieldConfiguration>,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub struct FieldConfiguration {
    pub name: String,
    #[serde(rename = "type")]
    pub field_type: FieldType,
    #[serde(default)]
    pub indexed: bool,
}

#[derive(Clone, Debug, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum FieldType {
    String,
}

impl Configuration {
    pub fn from_file(path: &std::path::Path) -> Result<Self, error::Error> {
        let content = std::fs::read_to_string(path)?;
        let configuration: Configuration = toml::from_str(&content)
            .map_err(|error| error::Error::from_message(error.to_string()))?;
        Ok(configuration)
    }
}
