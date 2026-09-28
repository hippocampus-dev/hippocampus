const CHALLENGE_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(600);

pub struct Client {
    mutex: tokio::sync::Mutex<()>,
    url: String,
    listen_port: u16,
}

impl Client {
    pub fn new(address: &str, listen_port: u16) -> Self {
        Client {
            mutex: tokio::sync::Mutex::new(()),
            url: address.to_string(),
            listen_port,
        }
    }

    pub async fn get_value(
        &self,
        cookie_name: &str,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let _guard = self.mutex.lock().await;

        let cookie = self.restore(cookie_name);
        if let (Some(value), Some(expires)) = (cookie.get("value"), cookie.get("expires"))
            && let Ok(t) = chrono::DateTime::parse_from_rfc3339(expires)
            && t.with_timezone(&chrono::Utc) > chrono::Utc::now()
        {
            return Ok(value.clone());
        }
        let cookie = self.challenge(cookie_name).await?;
        self.save(cookie_name, &cookie)?;
        cookie.get("value").cloned().ok_or("value not found".into())
    }

    pub async fn challenge(
        &self,
        cookie_name: &str,
    ) -> Result<
        std::collections::HashMap<String, String>,
        Box<dyn std::error::Error + Send + Sync + 'static>,
    > {
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
        let server = hyper::Server::try_bind(
            &format!("127.0.0.1:{}", self.listen_port).parse::<std::net::SocketAddr>()?,
        )?
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

        let mut u = url::Url::parse(&self.url)?;
        {
            let mut queries = u.query_pairs_mut();
            queries.append_pair(
                "redirect_url",
                &format!("http://127.0.0.1:{}", local_addr.port()),
            );
            queries.append_pair("cookie_name", cookie_name);
        }
        let uri: String = u.into();

        eprintln!("Please visit this URL to authorize this application: {uri}");
        std::thread::spawn(move || {
            let _ = webbrowser::open(&uri);
        });

        let cookie = tokio::select! {
            received = rx => received?,
            () = tokio::time::sleep(CHALLENGE_TIMEOUT) => {
                return Err(format!(
                    "no callback on http://127.0.0.1:{} within {CHALLENGE_TIMEOUT:?}",
                    local_addr.port()
                )
                .into());
            }
        };
        let _ = shutdown_tx.send(());
        let _ = server_handle.await;
        Ok(cookie)
    }

    pub fn restore(&self, cookie_name: &str) -> std::collections::HashMap<String, String> {
        if let Ok(content) =
            std::fs::read_to_string(format!("{}/{}", self.directory(), cookie_name))
            && let Ok(map) =
                serde_json::from_str::<std::collections::HashMap<String, String>>(&content)
        {
            return map;
        }
        std::collections::HashMap::new()
    }

    pub fn save(
        &self,
        cookie_name: &str,
        cookie: &std::collections::HashMap<String, String>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let b = serde_json::to_string(cookie)?;
        let d = self.directory();
        std::fs::create_dir_all(&d)?;
        std::fs::write(format!("{d}/{cookie_name}"), b)?;
        Ok(())
    }

    pub fn directory(&self) -> String {
        if let Ok(xdg_data_home) = std::env::var("XDG_DATA_HOME") {
            let mut pb = std::path::PathBuf::from(xdg_data_home);
            pb.push("bakery");
            return pb.to_string_lossy().into_owned();
        }

        if let Ok(home) = std::env::var("HOME") {
            let mut pb = std::path::PathBuf::from(home);
            pb.push(".local");
            pb.push("share");
            pb.push("bakery");
            return pb.to_string_lossy().into_owned();
        }

        let mut tmp = std::env::temp_dir();
        tmp.push("bakery");
        tmp.to_string_lossy().into_owned()
    }
}
