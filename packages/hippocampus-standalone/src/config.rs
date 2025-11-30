#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) struct HippocampusConfig {
    #[serde(rename = "TokenStorage")]
    pub(crate) token_storage: TokenStorageConfig,
    #[serde(rename = "DocumentStorage")]
    pub(crate) document_storage: DocumentStorageConfig,
}

#[allow(clippy::upper_case_acronyms)]
#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) enum TokenStorageKind {
    File,
    GCS,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) struct TokenStorageConfig {
    pub(crate) kind: TokenStorageKind,
    #[serde(rename = "File")]
    pub(crate) file: Option<FileConfig>,
    #[serde(rename = "GCS")]
    pub(crate) gcs: Option<GCSConfig>,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) struct FileConfig {
    pub(crate) path: Option<std::path::PathBuf>,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) struct GCSConfig {
    pub(crate) bucket: Option<String>,
    pub(crate) prefix: Option<String>,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) enum DocumentStorageKind {
    File,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub(crate) struct DocumentStorageConfig {
    pub(crate) kind: DocumentStorageKind,
    #[serde(rename = "File")]
    pub(crate) file: Option<FileConfig>,
}
