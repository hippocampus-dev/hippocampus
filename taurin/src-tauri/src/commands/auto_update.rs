use base64::Engine;
use tauri_plugin_store::StoreExt;

const AUTHORIZE_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(600);

pub async fn authorize(
    app_handle: &tauri::AppHandle,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let mut state_bytes = [0u8; 16];
    getrandom::getrandom(&mut state_bytes)?;
    let state = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(state_bytes);

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
    let server =
        hyper::Server::try_bind(&"127.0.0.1:0".parse::<std::net::SocketAddr>()?)?.serve(service_fn);
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
    let mut u = url::Url::parse(&format!("{}/login", crate::commands::TAURI_RELEASES_URL))?;
    {
        let mut queries = u.query_pairs_mut();
        queries.append_pair("redirect_uri", &redirect_uri);
        queries.append_pair("state", &state);
    }
    let uri: String = u.into();

    std::thread::spawn(move || {
        let _ = webbrowser::open(&uri);
    });

    let params = tokio::select! {
        received = rx => received?,
        () = tokio::time::sleep(AUTHORIZE_TIMEOUT) => {
            return Err(format!("no callback on {redirect_uri} within {AUTHORIZE_TIMEOUT:?}").into());
        }
    };
    let _ = shutdown_tx.send(());
    let _ = server_handle.await;

    if params.get("state") != Some(&state) {
        return Err("state mismatch in callback".into());
    }
    let token = params.get("token").ok_or("missing token in callback")?;

    let store = app_handle
        .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
        .build();
    store.set("Auto Update#Token", token.as_str());
    store.save()?;

    Ok(())
}
