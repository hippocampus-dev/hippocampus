use std::ops::{Add, Sub};
use std::str::FromStr;

use rsa::pkcs8::DecodePrivateKey;
use tracing_opentelemetry::OpenTelemetrySpanExt;

#[cfg(debug_assertions)]
use elapsed::prelude::*;
use error::context::Context;

pub mod strategy {
    pub use retry::strategy::*;
}

#[derive(Debug)]
pub enum Error {
    NotFound,
    ServiceUnavailable,
    InvalidStatusCode(u16, String),
}

impl std::error::Error for Error {}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::NotFound => {
                write!(f, "not found")
            }
            Error::ServiceUnavailable => {
                write!(f, "service unavailable")
            }
            Error::InvalidStatusCode(i, body) => {
                write!(f, "invalid status code: {i} body: {body}")
            }
        }
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct ServiceAccountKey {
    #[serde(rename = "type")]
    _type: String,
    project_id: String,
    private_key_id: String,
    private_key: String,
    client_email: String,
    client_id: String,
    auth_uri: String,
    token_uri: String,
    auth_provider_x509_cert_url: String,
    client_x509_cert_url: String,
}

/// <https://cloud.google.com/storage/docs/authentication?hl=ja#oauth-scopes>
#[derive(Clone, Debug, PartialEq, Eq, std::hash::Hash)]
enum Scope {
    ReadOnly,
    ReadWrite,
    #[allow(dead_code)]
    FullControl,
    #[allow(dead_code)]
    CloudPlatformReadOnly,
    #[allow(dead_code)]
    CloudPlatform,
}

impl AsRef<str> for Scope {
    fn as_ref(&self) -> &str {
        match self {
            Scope::ReadOnly => "https://www.googleapis.com/auth/devstorage.read_only",
            Scope::ReadWrite => "https://www.googleapis.com/auth/devstorage.read_write",
            Scope::FullControl => "https://www.googleapis.com/auth/devstorage.full_control",
            Scope::CloudPlatformReadOnly => {
                "https://www.googleapis.com/auth/cloud-platform.read-only"
            }
            Scope::CloudPlatform => "https://www.googleapis.com/auth/cloud-platform",
        }
    }
}

#[derive(Clone, Debug, jwt::Encode, serde::Serialize, serde::Deserialize)]
struct Claims {
    #[serde(flatten)]
    base: jwt::Claims,
    scope: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct AssertionBody {
    grant_type: String,
    assertion: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
struct TokenResponse {
    access_token: String,
    expires_in: u64,
}

#[derive(Clone, Debug)]
struct Token {
    access_token: String,
    expired_at: std::time::SystemTime,
}

impl From<TokenResponse> for Token {
    fn from(from: TokenResponse) -> Self {
        Self {
            access_token: from.access_token,
            expired_at: std::time::SystemTime::now()
                .add(std::time::Duration::from_secs(from.expires_in)),
        }
    }
}

impl Token {
    fn has_expired(&self) -> bool {
        self.expired_at <= std::time::SystemTime::now().sub(std::time::Duration::from_secs(60))
    }
}

pub trait RetryStrategy:
    Iterator<Item = std::time::Duration> + std::fmt::Debug + Send + Sync
{
    fn clone_box(&self) -> Box<dyn RetryStrategy>;
}

impl<T> RetryStrategy for T
where
    T: Iterator<Item = std::time::Duration> + std::fmt::Debug + Clone + Send + Sync + 'static,
{
    fn clone_box(&self) -> Box<dyn RetryStrategy> {
        Box::new(self.clone())
    }
}

impl Clone for Box<dyn RetryStrategy> {
    fn clone(&self) -> Self {
        self.as_ref().clone_box()
    }
}

#[derive(Clone, Debug)]
pub struct Client {
    inner: Inner,
    service_account_key: ServiceAccountKey,
    cache: std::sync::Arc<tokio::sync::RwLock<std::collections::HashMap<Scope, Token>>>,
    host: hyper::http::Uri,
    without_authentication: bool,
    retry_strategy: Box<dyn RetryStrategy>,
}

struct HeaderMapWrapper<'a>(pub &'a mut hyper::http::HeaderMap);

impl<'a> opentelemetry::propagation::Injector for HeaderMapWrapper<'a> {
    fn set(&mut self, key: &str, value: String) {
        if let Ok(k) = hyper::http::header::HeaderName::from_str(key)
            && let Ok(v) = hyper::http::header::HeaderValue::from_str(&value)
        {
            self.0.insert(k, v);
        }
    }
}

#[allow(clippy::upper_case_acronyms)]
#[derive(Clone, Debug)]
enum Inner {
    HTTP(hyper::Client<hyper::client::connect::HttpConnector>),
    #[cfg(feature = "use-rustls")]
    Rustls(hyper::Client<hyper_rustls::HttpsConnector<hyper::client::connect::HttpConnector>>),
    #[cfg(feature = "use-openssl")]
    OpenSSL(hyper::Client<hyper_openssl::HttpsConnector<hyper::client::connect::HttpConnector>>),
}

impl Inner {
    fn request(&self, mut req: hyper::Request<hyper::Body>) -> hyper::client::ResponseFuture {
        opentelemetry::global::get_text_map_propagator(|propagator| {
            propagator.inject_context(
                &tracing::Span::current().context(),
                &mut HeaderMapWrapper(req.headers_mut()),
            )
        });
        match self {
            Inner::HTTP(client) => client.request(req),
            #[cfg(feature = "use-rustls")]
            Inner::Rustls(client) => client.request(req),
            #[cfg(feature = "use-openssl")]
            Inner::OpenSSL(client) => client.request(req),
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct Builder {
    pool_idle_timeout: Option<std::time::Duration>,
    pool_max_idle_per_host: Option<usize>,
    connect_timeout: Option<std::time::Duration>,
    retry_strategy: Option<Box<dyn RetryStrategy>>,
}

impl Builder {
    pub fn set_pool_idle_timeout(&mut self, pool_idle_timeout: std::time::Duration) -> &mut Self {
        self.pool_idle_timeout = Some(pool_idle_timeout);
        self
    }

    pub fn pool_max_idle_per_host(&mut self, pool_max_idle_per_host: usize) -> &mut Self {
        self.pool_max_idle_per_host = Some(pool_max_idle_per_host);
        self
    }

    pub fn set_connect_timeout(&mut self, connect_timeout: std::time::Duration) -> &mut Self {
        self.connect_timeout = Some(connect_timeout);
        self
    }

    pub fn set_retry_strategy(&mut self, retry_strategy: Box<dyn RetryStrategy>) -> &mut Self {
        self.retry_strategy = Some(retry_strategy);
        self
    }

    #[allow(unused_variables)]
    pub fn build(&self, service_account_key: ServiceAccountKey) -> Result<Client, error::Error> {
        let mut connector = hyper::client::HttpConnector::new();
        connector.set_connect_timeout(self.connect_timeout);
        let mut builder = hyper::client::Client::builder();
        if let Some(pool_idle_timeout) = self.pool_idle_timeout {
            builder.pool_idle_timeout(pool_idle_timeout);
        }
        if let Some(pool_max_idle_per_host) = self.pool_max_idle_per_host {
            builder.pool_max_idle_per_host(pool_max_idle_per_host);
        }
        let inner = Inner::HTTP(builder.build(connector));

        #[cfg(feature = "use-rustls")]
        let inner = {
            use hyper_rustls::ConfigBuilderExt;
            let mut http = hyper::client::HttpConnector::new();
            http.set_connect_timeout(self.connect_timeout);
            http.enforce_http(false);
            let mut config = rustls::ClientConfig::builder()
                .with_safe_defaults()
                .with_native_roots()
                .with_no_client_auth();
            config.alpn_protocols = vec![b"h2".to_vec(), b"http/1.1".to_vec()];
            config.key_log = std::sync::Arc::new(rustls::KeyLogFile::new());
            let connector = hyper_rustls::HttpsConnector::from((http, config));
            let mut builder = hyper::client::Client::builder();
            if let Some(pool_idle_timeout) = self.pool_idle_timeout {
                builder.pool_idle_timeout(pool_idle_timeout);
            }
            if let Some(pool_max_idle_per_host) = self.pool_max_idle_per_host {
                builder.pool_max_idle_per_host(pool_max_idle_per_host);
            }
            Inner::Rustls(builder.build(connector))
        };

        #[cfg(feature = "use-openssl")]
        let inner = {
            use std::io::Write;
            let mut http = hyper::client::HttpConnector::new();
            http.set_connect_timeout(self.connect_timeout);
            http.enforce_http(false);
            let mut connector = openssl::ssl::SslConnector::builder(openssl::ssl::SslMethod::tls())
                .context("error occurred while building openssl::ssl::SslConnector")?;
            connector
                .set_alpn_protos(b"\x02h2\x08http/1.1")
                .context("error occurred while configuring alpn protocols")?;
            connector.set_keylog_callback(|_, s| {
                if let Ok(path) = std::env::var("SSLKEYLOGFILE") {
                    if let Ok(mut file) = std::fs::OpenOptions::new()
                        .append(true)
                        .create(true)
                        .open(path)
                    {
                        let _ = writeln!(file, "{}", s);
                    }
                }
            });
            let connector = hyper_openssl::HttpsConnector::with_connector(http, connector)
                .context("error occurred while building hyper_openssl::HttpsConnector")?;
            let mut builder = hyper::client::Client::builder();
            if let Some(pool_idle_timeout) = self.pool_idle_timeout {
                builder.pool_idle_timeout(pool_idle_timeout);
            }
            if let Some(pool_max_idle_per_host) = self.pool_max_idle_per_host {
                builder.pool_max_idle_per_host(pool_max_idle_per_host);
            }
            Inner::OpenSSL(builder.build(connector))
        };

        if let Ok(host) = std::env::var("STORAGE_EMULATOR_HOST") {
            return Ok(Client {
                inner,
                service_account_key,
                cache: std::sync::Arc::new(tokio::sync::RwLock::new(
                    std::collections::HashMap::new(),
                )),
                host: host.parse()?,
                without_authentication: true,
                retry_strategy: self.retry_strategy.clone().unwrap_or(Box::new(
                    strategy::JitteredExponentialBackoff::new(std::time::Duration::from_millis(10))
                        .take(3),
                )),
            });
        }

        Ok(Client {
            inner,
            service_account_key,
            cache: std::sync::Arc::new(tokio::sync::RwLock::new(std::collections::HashMap::new())),
            host: "https://www.googleapis.com".parse()?,
            without_authentication: false,
            retry_strategy: self.retry_strategy.clone().unwrap_or(Box::new(
                strategy::JitteredExponentialBackoff::new(std::time::Duration::from_millis(10))
                    .take(3),
            )),
        })
    }
}

impl Client {
    pub fn builder() -> Builder {
        Builder::default()
    }

    /// <https://datatracker.ietf.org/doc/html/rfc7523>
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn auth(&self, scope: Scope) -> Result<Token, error::Error> {
        {
            if let Some(token) = self.cache.read().await.get(&scope)
                && !token.has_expired()
            {
                return Ok(token.clone());
            }
        }

        let header = jwt::Header {
            alg: jwt::Algorithm::RS256,
            jku: None,
            jwk: None,
            kid: None,
            x5u: None,
            x5c: None,
            x5t: None,
            x5t_s256: None,
            typ: Some("JWT".to_string()),
            cty: None,
            crit: None,
        };

        let now = std::time::SystemTime::now();
        let iat = now.duration_since(std::time::UNIX_EPOCH)?;
        let expire = now
            .add(std::time::Duration::from_secs(60))
            .duration_since(std::time::UNIX_EPOCH)?;

        let claims = Claims {
            base: jwt::Claims {
                iss: Some(self.service_account_key.client_email.clone()),
                sub: None,
                aud: Some(self.service_account_key.token_uri.clone()),
                exp: Some(format!("{}", expire.as_secs())),
                nbf: None,
                iat: Some(format!("{}", iat.as_secs())),
                jti: None,
            },
            scope: scope.as_ref().to_string(),
        };

        let private_key = rsa::RsaPrivateKey::from_pkcs8_pem(&self.service_account_key.private_key)
            .context("error occurred while generating rsa::RsaPrivateKey")?;
        let assertion = jwt::sign_with_rsa(&private_key, &header, &claims)
            .context("error occurred in jwt::sign_with_rsa")?;
        let body = serde_json::to_string(&AssertionBody {
            grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer".to_string(),
            assertion,
        })?;
        let request = hyper::Request::builder()
            .method(hyper::Method::POST)
            .uri(&self.service_account_key.token_uri)
            .header(hyper::header::CONTENT_TYPE, "application/json")
            .header(hyper::header::CONTENT_LENGTH, body.len())
            .body(hyper::Body::from(body))
            .context("error occurred while encoding a request body")?;
        let mut response = self.inner.request(request).await?;
        let response_body_bytes = hyper::body::to_bytes(response.body_mut())
            .await
            .context("error occurred while encoding a response body")?;
        let token_response: TokenResponse = serde_json::from_slice(&response_body_bytes[..])?;
        let token = Token::from(token_response);

        {
            self.cache.write().await.insert(scope, token.clone());
        }

        Ok(token)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    pub async fn download<S>(&self, bucket: S, object: S) -> Result<Vec<u8>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug + Sync,
    {
        let response_body_bytes: hyper::body::Bytes =
            retry::spawn(self.retry_strategy.clone(), || async {
                hedged::spawn(std::time::Duration::from_millis(50), 1, || async {
                    let mut builder =
                        hyper::Request::builder()
                            .method(hyper::Method::GET)
                            .uri(format!(
                                "{}storage/v1/b/{}/o/{}?alt=media",
                                self.host,
                                percent_encoding::percent_encode(
                                    bucket.as_ref().as_bytes(),
                                    percent_encoding::NON_ALPHANUMERIC
                                ),
                                percent_encoding::percent_encode(
                                    object.as_ref().as_bytes(),
                                    percent_encoding::NON_ALPHANUMERIC
                                )
                            ));
                    if !self.without_authentication {
                        let token = self
                            .auth(Scope::ReadOnly)
                            .await
                            .context(
                                "error occurred while authenticating to GCS with Scope::ReadOnly",
                            )
                            .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                        builder = builder.header(
                            hyper::header::AUTHORIZATION,
                            &format!("Bearer {}", token.access_token),
                        );
                    }
                    let request = builder
                        .body(hyper::Body::empty())
                        .context("error occurred while encoding a request body")
                        .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                    let mut response = match self.inner.request(request).await {
                        Ok(response) => response,
                        Err(e) => return handle_hyper_error(e),
                    };
                    let response_body_bytes = hyper::body::to_bytes(response.body_mut())
                        .await
                        .context("error occurred while encoding a response body")
                        .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                    let status = response.status();
                    if !status.is_success() {
                        return handle_hyper_status_code(status, response_body_bytes);
                    }
                    Ok(response_body_bytes)
                })
                .await
            })
            .await?;
        Ok(response_body_bytes.to_vec())
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    pub async fn upload<S, T>(
        &self,
        bucket: S,
        object: S,
        content: T,
    ) -> Result<hyper::body::Bytes, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug + Sync,
        T: AsRef<[u8]> + std::fmt::Debug + Sync,
    {
        let response_body_bytes: hyper::body::Bytes =
            retry::spawn(self.retry_strategy.clone(), || async {
                hedged::spawn(std::time::Duration::from_millis(50), 1, || async {
                    let mut builder =
                        hyper::Request::builder()
                            .method(hyper::Method::POST)
                            .uri(format!(
                                "{}storage/v1/b/{}/o?name={}&uploadType=media",
                                self.host,
                                percent_encoding::percent_encode(
                                    bucket.as_ref().as_bytes(),
                                    percent_encoding::NON_ALPHANUMERIC
                                ),
                                percent_encoding::percent_encode(
                                    object.as_ref().as_bytes(),
                                    percent_encoding::NON_ALPHANUMERIC
                                )
                            ));
                    if !self.without_authentication {
                        let token = self
                            .auth(Scope::ReadWrite)
                            .await
                            .context(
                                "error occurred while authenticating to GCS with Scope::ReadWrite",
                            )
                            .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                        builder = builder.header(
                            hyper::header::AUTHORIZATION,
                            &format!("Bearer {}", token.access_token),
                        )
                    }
                    let request = builder
                        .header(hyper::header::CONTENT_TYPE, "application/octet-stream")
                        .body(hyper::Body::from(content.as_ref().to_owned()))
                        .context("error occurred while encoding a request body")
                        .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                    let mut response = match self.inner.request(request).await {
                        Ok(response) => response,
                        Err(e) => return handle_hyper_error(e),
                    };
                    let response_body_bytes = hyper::body::to_bytes(response.body_mut())
                        .await
                        .context("error occurred while encoding a response body")
                        .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                    let status = response.status();
                    if !status.is_success() {
                        return handle_hyper_status_code(status, response_body_bytes);
                    }
                    Ok(response_body_bytes)
                })
                .await
            })
            .await?;
        Ok(response_body_bytes)
    }
}

fn handle_hyper_error(error: hyper::Error) -> Result<hyper::body::Bytes, retry::RetryError> {
    use error::context::ContextableError;
    // https://www.rfc-editor.org/rfc/rfc2616#section-8.1.4
    if error.is_incomplete_message() {
        return Err(retry::RetryError::from_retriable_error(
            Box::new(error)
                .add_context("EOF error occurred while requesting to GCS")
                .into(),
        ));
    }
    // is_connect() cannot be determined to be retriable. https://github.com/hyperium/hyper/issues/1131#issuecomment-362379005
    handle_error(error)
}

fn handle_error<E>(error: E) -> Result<hyper::body::Bytes, retry::RetryError>
where
    E: std::error::Error + Send + Sync + 'static + error::context::ContextableError,
{
    if let Some(io_e) = error
        .source()
        .and_then(|e| e.downcast_ref::<std::io::Error>())
    {
        // connect(2) may return ECONNREFUSED
        // https://elixir.bootlin.com/linux/v6.0.12/source/net/ipv4/tcp_output.c#L3869
        // tcp_reset may return ECONNREFUSED, EPIPE, ECONNRESET
        // https://elixir.bootlin.com/linux/v6.0.12/source/net/ipv4/tcp_input.c#L4311
        match io_e.kind() {
            std::io::ErrorKind::ConnectionRefused
            | std::io::ErrorKind::BrokenPipe
            | std::io::ErrorKind::ConnectionReset => {
                return Err(retry::RetryError::from_retriable_error(
                    error
                        .add_context("reset error occurred while requesting to GCS")
                        .into(),
                ));
            }
            std::io::ErrorKind::TimedOut => {
                return Err(retry::RetryError::from_retriable_error(
                    error
                        .add_context("timeout error occurred while requesting to GCS")
                        .into(),
                ));
            }
            _ => {}
        }
    }
    Err(retry::RetryError::from_unexpected_error(
        error
            .add_context("error occurred while requesting to GCS")
            .into(),
    ))
}

fn handle_hyper_status_code(
    status_code: hyper::StatusCode,
    response_body_bytes: hyper::body::Bytes,
) -> Result<hyper::body::Bytes, retry::RetryError> {
    match status_code {
        hyper::http::StatusCode::NOT_FOUND => Err(retry::RetryError::from_unexpected_error(
            Box::new(Error::NotFound),
        )),
        hyper::http::StatusCode::CONFLICT
        | hyper::http::StatusCode::TOO_MANY_REQUESTS
        | hyper::http::StatusCode::BAD_GATEWAY
        | hyper::http::StatusCode::SERVICE_UNAVAILABLE
        | hyper::http::StatusCode::GATEWAY_TIMEOUT => Err(retry::RetryError::from_retriable_error(
            Box::new(Error::ServiceUnavailable),
        )),
        _ => Err(retry::RetryError::from_unexpected_error(Box::new(
            Error::InvalidStatusCode(
                status_code.into(),
                String::from_utf8_lossy(&response_body_bytes).to_string(),
            ),
        ))),
    }
}
