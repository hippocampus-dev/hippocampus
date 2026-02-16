use hyper::body::HttpBody;
use std::str::FromStr;
use tracing_opentelemetry::OpenTelemetrySpanExt;

use error::context::Context;

pub mod types;
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
    host: hyper::http::Uri,
    token: String,
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

fn get_openai_api_base() -> Result<hyper::http::Uri, hyper::http::uri::InvalidUri> {
    if let Ok(base) = std::env::var("OPENAI_BASE_URL") {
        return base.parse();
    }
    "https://api.openai.com/v1".parse()
}

#[derive(Clone, Default)]
pub struct Builder {
    pool_idle_timeout: Option<std::time::Duration>,
    pool_max_idle_per_host: Option<usize>,
    connect_timeout: Option<std::time::Duration>,
    retry_strategy: Option<Box<dyn RetryStrategy>>,
    openai_api_base: Option<hyper::http::Uri>,
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

    pub fn set_openai_api_base(&mut self, openai_api_base: hyper::http::Uri) -> &mut Self {
        self.openai_api_base = Some(openai_api_base);
        self
    }

    #[allow(unused_variables)]
    pub fn build(&self, token: String) -> Result<Client, error::Error> {
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
                        let _ = writeln!(file, "{s}");
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

        Ok(Client {
            inner,
            host: self
                .openai_api_base
                .clone()
                .unwrap_or(get_openai_api_base()?),
            token,
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

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, content)))]
    pub async fn post<S, T>(&self, path: S, content: T) -> Result<Vec<u8>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug,
        T: AsRef<[u8]> + std::fmt::Debug,
    {
        let response_body_bytes: hyper::body::Bytes =
            retry::spawn(self.retry_strategy.clone(), || async {
                let request = hyper::Request::builder()
                    .method(hyper::Method::POST)
                    .uri(format!("{}{}", self.host, path.as_ref()))
                    .header(hyper::header::CONTENT_TYPE, "application/json")
                    .header(hyper::header::COOKIE, &self.token)
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
            .await?;
        Ok(response_body_bytes.to_vec())
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, content)))]
    pub fn post_stream<S, T>(
        &self,
        path: S,
        content: T,
    ) -> impl futures::Stream<Item = Result<hyper::body::Bytes, error::Error>>
    where
        S: AsRef<str> + std::fmt::Debug,
        T: AsRef<[u8]> + std::fmt::Debug,
    {
        let cloned_path = path.as_ref().to_owned();
        let cloned_content = content.as_ref().to_owned();
        let cloned_inner = self.inner.clone();
        let cloned_host = self.host.clone();
        let cloned_token = self.token.clone();
        let clone_retry_strategy = self.retry_strategy.clone();

        async_stream::stream! {
            let response = match retry::spawn(clone_retry_strategy, || async {
                let request = hyper::Request::builder()
                    .method(hyper::Method::POST)
                    .uri(format!("{cloned_host}{cloned_path}"))
                    .header(hyper::header::CONTENT_TYPE, "application/json")
                    .header(hyper::header::COOKIE, &cloned_token)
                    .body(hyper::Body::from(cloned_content.clone()))
                    .context("error occurred while encoding a request body")
                    .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                let response = match cloned_inner.request(request).await {
                    Ok(response) => response,
                    Err(e) => return handle_hyper_error(e),
                };
                let status = response.status();
                if !status.is_success() {
                    let response_body_bytes = hyper::body::to_bytes(response.into_body())
                        .await
                        .context("error occurred while encoding a response body")
                        .map_err(|e| retry::RetryError::from_unexpected_error(e.into()))?;
                    return handle_hyper_status_code(status, response_body_bytes);
                }
                Ok(response)
            }).await {
                Ok(response) => response,
                Err(e) => {
                    yield Err(error::Error::from(e));
                    return;
                }
            };

            let mut body = response.into_body();

            while let Some(chunk_result) = body.data().await {
                match chunk_result {
                    Ok(chunk) => yield Ok(chunk),
                    Err(e) => {
                        yield Err(error::Error::from(e));
                        return;
                    }
                }
            }
        }
    }
}

fn handle_hyper_error<T>(error: hyper::Error) -> Result<T, retry::RetryError> {
    use error::context::ContextableError;
    // https://www.rfc-editor.org/rfc/rfc2616#section-8.1.4
    if error.is_incomplete_message() {
        return Err(retry::RetryError::from_retriable_error(
            Box::new(error)
                .add_context("EOF error occurred while requesting to OpenAI")
                .into(),
        ));
    }
    // is_connect() cannot be determined to be retriable. https://github.com/hyperium/hyper/issues/1131#issuecomment-362379005
    handle_error(error)
}

fn handle_error<E, T>(error: E) -> Result<T, retry::RetryError>
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
                        .add_context("reset error occurred while requesting to OpenAI")
                        .into(),
                ));
            }
            std::io::ErrorKind::TimedOut => {
                return Err(retry::RetryError::from_retriable_error(
                    error
                        .add_context("timeout error occurred while requesting to OpenAI")
                        .into(),
                ));
            }
            _ => {}
        }
    }
    Err(retry::RetryError::from_unexpected_error(
        error
            .add_context("error occurred while requesting to OpenAI")
            .into(),
    ))
}

fn handle_hyper_status_code<T>(
    status_code: hyper::StatusCode,
    response_body_bytes: hyper::body::Bytes,
) -> Result<T, retry::RetryError> {
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
