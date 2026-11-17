use base64::Engine;
use sha2::Digest;

const AUTHORIZE_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(600);

#[derive(serde::Deserialize)]
pub struct TokenResponse {
    pub access_token: String,
    pub expires_in: Option<u64>,
    pub refresh_token: Option<String>,
}

#[derive(serde::Serialize, serde::Deserialize)]
pub struct Token {
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub expires_at: u64,
}

pub struct Client {
    mutex: tokio::sync::Mutex<()>,
    url: String,
}

impl Client {
    pub fn new(url: &str) -> Self {
        Client {
            mutex: tokio::sync::Mutex::new(()),
            url: url.to_string(),
        }
    }

    pub async fn get_access_token<F, Fut>(
        &self,
        name: &str,
        scope: &str,
        cookie: F,
    ) -> Result<String, error::Error>
    where
        F: FnOnce() -> Fut,
        Fut: std::future::Future<Output = Result<String, error::Error>>,
    {
        let _guard = self.mutex.lock().await;

        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)?
            .as_secs();
        let stored = self.restore(name);
        if let Some(token) = &stored
            && token.expires_at > now
        {
            return Ok(token.access_token.clone());
        }

        let cookie = cookie().await?;
        let response = match stored.as_ref().and_then(|t| t.refresh_token.as_deref()) {
            Some(refresh_token) => match self.refresh(&cookie, refresh_token).await? {
                Some(response) => response,
                None => self.authorize(&cookie, scope).await?,
            },
            None => self.authorize(&cookie, scope).await?,
        };
        let token = Token {
            access_token: response.access_token,
            // https://www.rfc-editor.org/rfc/rfc6749#section-6
            refresh_token: response
                .refresh_token
                .or(stored.and_then(|t| t.refresh_token)),
            expires_at: now + response.expires_in.unwrap_or(0),
        };
        self.save(name, &token)?;
        Ok(token.access_token)
    }

    pub async fn authorize(
        &self,
        cookie: &str,
        scope: &str,
    ) -> Result<TokenResponse, error::Error> {
        let mut state_bytes = [0u8; 16];
        getrandom::getrandom(&mut state_bytes)?;
        let state = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(state_bytes);

        let mut code_verifier_bytes = [0u8; 32];
        getrandom::getrandom(&mut code_verifier_bytes)?;
        let code_verifier =
            base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(code_verifier_bytes);

        let (tx, rx) = tokio::sync::oneshot::channel::<std::collections::HashMap<String, String>>();

        let tx = std::sync::Arc::new(tokio::sync::Mutex::new(Some(tx)));

        let service_fn = hyper::service::make_service_fn(
            move |_socket: &hyper::server::conn::AddrStream| {
                let tx = std::sync::Arc::clone(&tx);
                async move {
                    Ok::<_, std::convert::Infallible>(hyper::service::service_fn(
                        move |request: hyper::Request<hyper::Body>| {
                            let tx = std::sync::Arc::clone(&tx);
                            async move {
                                let query = request.uri().query().unwrap_or("");
                                let params: std::collections::HashMap<String, String> =
                                    url::form_urlencoded::parse(query.as_bytes())
                                        .into_owned()
                                        .collect();
                                if let Some(tx) = tx.lock().await.take() {
                                    let _ = tx.send(params);
                                }
                                let response = hyper::Response::builder()
                                    .status(200)
                                    .header("Content-Type", "text/html")
                                    .body(hyper::Body::from(
                                        r#"<script>window.open("about:blank","_self").close()</script>"#,
                                    ))
                                    .unwrap();
                                Ok::<_, std::convert::Infallible>(response)
                            }
                        },
                    ))
                }
            },
        );
        let server = hyper::Server::try_bind(&"127.0.0.1:0".parse::<std::net::SocketAddr>()?)?
            .serve(service_fn);
        let local_addr = server.local_addr();

        let (shutdown_tx, shutdown_rx) = tokio::sync::oneshot::channel::<()>();
        let server_handle = tokio::spawn(async move {
            let _ = server
                .with_graceful_shutdown(async move {
                    let _ = shutdown_rx.await;
                })
                .await;
        });

        let redirect_uri = format!("http://127.0.0.1:{}", local_addr.port());
        let mut u = url::Url::parse(&format!("{}/authorize", self.url))?;
        {
            let mut queries = u.query_pairs_mut();
            queries.append_pair("response_type", "code");
            queries.append_pair("redirect_uri", &redirect_uri);
            queries.append_pair("state", &state);
            queries.append_pair("code_challenge", &code_challenge(&code_verifier));
            queries.append_pair("code_challenge_method", "S256");
            queries.append_pair("scope", scope);
        }
        let uri: String = u.into();

        eprintln!("Please visit this URL to authorize this application: {uri}");
        std::thread::spawn(move || {
            let _ = webbrowser::open(&uri);
        });

        let params = tokio::select! {
            received = rx => received?,
            () = tokio::time::sleep(AUTHORIZE_TIMEOUT) => {
                error::bail!("no callback on {redirect_uri} within {AUTHORIZE_TIMEOUT:?}");
            }
        };
        let _ = shutdown_tx.send(());
        let _ = server_handle.await;

        if params.get("state") != Some(&state) {
            error::bail!("state mismatch in callback");
        }
        let code = params
            .get("code")
            .ok_or_else(|| error::error!("missing code in callback"))?;

        self.request_token(
            cookie,
            &[
                ("grant_type", "authorization_code"),
                ("code", code),
                ("redirect_uri", &redirect_uri),
                ("code_verifier", &code_verifier),
            ],
        )
        .await?
        .ok_or_else(|| error::error!("authorization code was rejected"))
    }

    pub async fn refresh(
        &self,
        cookie: &str,
        refresh_token: &str,
    ) -> Result<Option<TokenResponse>, error::Error> {
        self.request_token(
            cookie,
            &[
                ("grant_type", "refresh_token"),
                ("refresh_token", refresh_token),
            ],
        )
        .await
    }

    pub fn restore(&self, name: &str) -> Option<Token> {
        let content = std::fs::read_to_string(format!("{}/{}", self.directory(), name)).ok()?;
        serde_json::from_str(&content).ok()
    }

    pub fn save(&self, name: &str, token: &Token) -> Result<(), error::Error> {
        let b = serde_json::to_string(token)?;
        let d = self.directory();
        std::fs::create_dir_all(&d)?;
        std::fs::write(format!("{d}/{name}"), b)?;
        Ok(())
    }

    pub fn directory(&self) -> String {
        if let Ok(xdg_data_home) = std::env::var("XDG_DATA_HOME")
            && !xdg_data_home.is_empty()
        {
            let mut path = std::path::PathBuf::from(xdg_data_home);
            path.push("oauth-bridge");
            return path.to_string_lossy().into_owned();
        }

        if let Some(home) = std::env::home_dir() {
            let mut path = std::path::PathBuf::from(home);
            path.push(".local");
            path.push("share");
            path.push("oauth-bridge");
            return path.to_string_lossy().into_owned();
        }

        let mut temporary_directory = std::env::temp_dir();
        temporary_directory.push("oauth-bridge");
        temporary_directory.to_string_lossy().into_owned()
    }

    async fn request_token(
        &self,
        cookie: &str,
        form: &[(&str, &str)],
    ) -> Result<Option<TokenResponse>, error::Error> {
        let body = url::form_urlencoded::Serializer::new(String::new())
            .extend_pairs(form)
            .finish();
        let request = hyper::Request::post(format!("{}/token", self.url))
            .header(
                hyper::header::CONTENT_TYPE,
                "application/x-www-form-urlencoded",
            )
            .header(hyper::header::COOKIE, cookie)
            .body(hyper::Body::from(body))?;

        let https = {
            use hyper_rustls::ConfigBuilderExt;
            let config = rustls::ClientConfig::builder()
                .with_safe_defaults()
                .with_native_roots()
                .with_no_client_auth();
            hyper_rustls::HttpsConnectorBuilder::new()
                .with_tls_config(config)
                .https_only()
                .enable_http1()
                .build()
        };
        let client: hyper::Client<_, hyper::Body> = hyper::Client::builder().build(https);

        let response = client.request(request).await?;
        let status = response.status();
        let bytes = hyper::body::to_bytes(response.into_body()).await?;

        // https://www.rfc-editor.org/rfc/rfc6749#section-5.2
        if status == hyper::StatusCode::BAD_REQUEST {
            return Ok(None);
        }
        if !status.is_success() {
            error::bail!(
                "failed to request token: status {}: {}",
                status,
                String::from_utf8_lossy(&bytes)
            );
        }

        Ok(Some(serde_json::from_slice(&bytes)?))
    }
}

// https://www.rfc-editor.org/rfc/rfc7636#section-4.2
fn code_challenge(code_verifier: &str) -> String {
    base64::engine::general_purpose::URL_SAFE_NO_PAD
        .encode(sha2::Sha256::digest(code_verifier.as_bytes()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_code_challenge() {
        assert_eq!(
            code_challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"),
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
        );
    }
}
