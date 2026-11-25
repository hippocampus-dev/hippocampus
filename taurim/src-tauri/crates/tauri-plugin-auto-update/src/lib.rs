use base64::Engine;
use hyper::body::HttpBody;
use tauri::Manager;
use tauri_plugin_store::StoreExt;

const TAURI_RELEASES_URL: &str = "https://tauri-releases-public.kaidotio.dev";
const LATEST_JSON_URL: &str = "https://github.com/hippocampus-dev/hippocampus/releases/latest/download/taurim_latest.json";
const SETTINGS_FILE_NAME: &str = if cfg!(debug_assertions) {
    "settings.dev.bin"
} else {
    "settings.bin"
};
const HTTP_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(30);
const MAX_REDIRECTS: usize = 10;
const APK_FILE_NAME: &str = "update.apk";
// Auto Backup skips Context.getNoBackupFilesDir(): https://developer.android.com/identity/data/autobackup#Files
const NO_BACKUP_DIRECTORY_NAME: &str = "no_backup";

struct AutoUpdate<R: tauri::Runtime>(tauri::plugin::PluginHandle<R>);

struct Downloaded(tokio::sync::watch::Sender<bool>);

#[derive(Debug, serde::Deserialize)]
struct Latest {
    version: semver::Version,
    platforms: std::collections::HashMap<String, Platform>,
}

#[derive(Debug, serde::Deserialize)]
struct Platform {
    signature: String,
    url: String,
}

async fn get<R: tauri::Runtime>(
    store: &tauri_plugin_store::Store<R>,
    url: &str,
    token: Option<&str>,
) -> Result<hyper::Body, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let https = {
        use hyper_rustls::ConfigBuilderExt;
        let config = rustls::ClientConfig::builder()
            .with_safe_defaults()
            .with_webpki_roots()
            .with_no_client_auth();
        hyper_rustls::HttpsConnectorBuilder::new()
            .with_tls_config(config)
            .https_only()
            .enable_http1()
            .build()
    };
    let client: hyper::Client<_, hyper::Body> = hyper::Client::builder().build(https);

    let mut url = url::Url::parse(url)?;
    let mut token = token;
    for _ in 0..MAX_REDIRECTS {
        let mut request = hyper::Request::get(url.as_str())
            // api.github.com answers a release asset URL with its metadata otherwise: https://docs.github.com/en/rest/releases/assets#get-a-release-asset
            .header(hyper::header::ACCEPT, "application/octet-stream")
            // Cloudflare's Browser Integrity Check challenges requests without one: https://developers.cloudflare.com/waf/tools/browser-integrity-check/
            .header(
                hyper::header::USER_AGENT,
                concat!(env!("CARGO_PKG_NAME"), "/", env!("CARGO_PKG_VERSION")),
            )
            .body(hyper::Body::empty())?;
        if let Some(token) = token {
            request.headers_mut().insert(
                hyper::header::AUTHORIZATION,
                hyper::header::HeaderValue::from_str(&format!("Bearer {token}"))?,
            );
        }

        let response = tokio::time::timeout(HTTP_TIMEOUT, client.request(request)).await??;
        // GitHub serves release downloads through a redirect, which hyper::Client does not follow: https://docs.github.com/en/rest/releases/assets#get-a-release-asset
        if response.status().is_redirection() {
            let location = response
                .headers()
                .get(hyper::header::LOCATION)
                .ok_or_else(|| {
                    format!("GET {url} returned {} without Location", response.status())
                })?
                .to_str()?;
            let next = url.join(location)?;
            if next.host_str() != url.host_str() {
                token = None;
            }
            url = next;
            continue;
        }
        if response.status() == hyper::StatusCode::UNAUTHORIZED {
            store.delete("Auto Update#Token");
            store.save()?;
        }
        if response.status() != hyper::StatusCode::OK {
            return Err(format!("GET {url} returned {}", response.status()).into());
        }
        return Ok(response.into_body());
    }
    Err(format!("GET {url} redirected more than {MAX_REDIRECTS} times").into())
}

async fn read(
    mut body: hyper::Body,
    writer: &mut impl std::io::Write,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    while let Some(chunk) = tokio::time::timeout(HTTP_TIMEOUT, body.data()).await? {
        writer.write_all(&chunk?)?;
    }
    Ok(())
}

pub fn requires_authorization() -> bool {
    LATEST_JSON_URL.starts_with(TAURI_RELEASES_URL)
}

pub fn has_token<R: tauri::Runtime>(app_handle: &tauri::AppHandle<R>) -> bool {
    app_handle
        .store_builder(SETTINGS_FILE_NAME)
        .build()
        .is_ok_and(|store| store.get("Auto Update#Token").is_some())
}

pub async fn update<R: tauri::Runtime>(
    app_handle: &tauri::AppHandle<R>,
) -> Result<bool, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build()?;
    if requires_authorization() && store.get("Auto Update#Token").is_none() {
        #[derive(serde::Serialize)]
        struct Request {
            url: String,
        }

        let mut state_bytes = [0u8; 16];
        getrandom::getrandom(&mut state_bytes)?;
        let state = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(state_bytes);

        let mut u = url::Url::parse(&format!("{TAURI_RELEASES_URL}/login"))?;
        {
            let mut queries = u.query_pairs_mut();
            queries.append_pair(
                "redirect_uri",
                &format!("{TAURI_RELEASES_URL}/taurim/callback"),
            );
            queries.append_pair("state", &state);
        }

        let params: std::collections::HashMap<String, String> = app_handle
            .state::<AutoUpdate<R>>()
            .0
            .run_mobile_plugin_async("authorize", Request { url: u.into() })
            .await?;

        if params.get("state") != Some(&state) {
            return Err("state mismatch in callback".into());
        }
        let token = params.get("token").ok_or("missing token in callback")?;
        store.set("Auto Update#Token", token.as_str());
        store.save()?;
    }
    let token = if requires_authorization() {
        let Some(serde_json::Value::String(token)) = store.get("Auto Update#Token") else {
            return Err("missing token in store".into());
        };
        Some(token)
    } else {
        None
    };

    let mut bytes = Vec::new();
    read(
        get(&store, LATEST_JSON_URL, token.as_deref()).await?,
        &mut bytes,
    )
    .await?;
    let latest: Latest = serde_json::from_slice(&bytes)?;
    let directory = app_handle
        .path()
        .app_local_data_dir()?
        .join(NO_BACKUP_DIRECTORY_NAME);
    let partial = directory.join(format!("{APK_FILE_NAME}.part"));
    if latest.version <= app_handle.package_info().version {
        for path in [directory.join(APK_FILE_NAME), partial] {
            match std::fs::remove_file(path) {
                Err(e) if e.kind() != std::io::ErrorKind::NotFound => return Err(e.into()),
                _ => {}
            }
        }
        return Ok(false);
    }
    let platform = latest
        .platforms
        .get("android")
        .ok_or("missing android in latest.json")?;

    std::fs::create_dir_all(&directory)?;
    read(
        get(&store, &platform.url, token.as_deref()).await?,
        &mut std::fs::File::create(&partial)?,
    )
    .await?;

    let public_key = minisign_verify::PublicKey::decode(&String::from_utf8(
        base64::engine::general_purpose::STANDARD.decode(env!("TAURIN_UPDATER_PUBKEY"))?,
    )?)?;
    let signature = minisign_verify::Signature::decode(&String::from_utf8(
        base64::engine::general_purpose::STANDARD.decode(&platform.signature)?,
    )?)?;
    public_key.verify(&std::fs::read(&partial)?, &signature, true)?;
    std::fs::rename(&partial, directory.join(APK_FILE_NAME))?;

    app_handle.state::<Downloaded>().0.send_replace(true);

    Ok(true)
}

#[tauri::command]
async fn wait_for_update<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    app.state::<Downloaded>()
        .0
        .subscribe()
        .wait_for(|downloaded| *downloaded)
        .await
        .map(|_| ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn install_update<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    #[derive(serde::Serialize)]
    struct Request {
        path: std::path::PathBuf,
    }

    let path = app
        .path()
        .app_local_data_dir()
        .map_err(|e| e.to_string())?
        .join(NO_BACKUP_DIRECTORY_NAME)
        .join(APK_FILE_NAME);
    app.state::<AutoUpdate<R>>()
        .0
        .run_mobile_plugin("installUpdate", Request { path })
        .map_err(|e| e.to_string())
}

pub fn init<R: tauri::Runtime>() -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("auto-update")
        .invoke_handler(tauri::generate_handler![wait_for_update, install_update])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.autoupdate", "AutoUpdatePlugin")
                .expect("failed to register auto update plugin");
            app.manage(AutoUpdate(handle));
            app.manage(Downloaded(tokio::sync::watch::channel(false).0));
            Ok(())
        })
        .build()
}
