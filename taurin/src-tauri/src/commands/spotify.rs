use tauri::{Emitter, Manager};
use tauri_plugin_store::StoreExt;

const SCOPE: &str = "user-read-currently-playing";

const REQUEST_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(10);

#[tauri::command]
#[ipc::estringify]
pub async fn get_spotify_currently_playing(
    app_handle: tauri::AppHandle,
) -> Result<Option<ipc::types::CurrentlyPlaying>, Box<dyn std::error::Error + Send + Sync + 'static>>
{
    let currently_playing = if internal::enabled(&app_handle) {
        internal::currently_playing().await
    } else {
        Ok(None)
    };

    if let Some(overlay) = app_handle.get_webview_window("spotify") {
        // The overlay may have been disabled while the request was in flight
        if internal::enabled(&app_handle) && !matches!(currently_playing, Ok(None)) {
            // GTK drops a window's position when it is hidden: https://github.com/GNOME/gtk/blob/gtk-3-24/gtk/gtkwindow.c
            if !overlay.is_visible().unwrap_or(false) {
                crate::commands::overlay::place(&app_handle, "spotify");
            }
            let _ = overlay.show();
        } else {
            let _ = overlay.hide();
        }
    }

    currently_playing
}

#[tauri::command]
pub fn has_spotify_token() -> bool {
    oauth_bridge::Client::new(crate::commands::SPOTIFY_OAUTH_BRIDGE_URL)
        .restore("spotify")
        .is_some()
}

pub fn apply_overlay_visibility<R>(app: &tauri::AppHandle<R>, enabled: bool)
where
    R: tauri::Runtime,
{
    if let Some(overlay) = app.get_webview_window("spotify") {
        if !enabled {
            let _ = overlay.hide();
            return;
        }

        crate::commands::overlay::place(app, "spotify");
        let _ = app.emit("spotify-enabled", ());
    }
}

mod internal {
    use super::*;

    #[derive(serde::Deserialize)]
    struct CurrentlyPlaying {
        is_playing: bool,
        progress_ms: Option<u64>,
        item: Option<Track>,
    }

    #[derive(serde::Deserialize)]
    struct Track {
        name: String,
        artists: Vec<Artist>,
        album: Album,
        duration_ms: u64,
    }

    #[derive(serde::Deserialize)]
    struct Artist {
        name: String,
    }

    #[derive(serde::Deserialize)]
    struct Album {
        name: String,
        images: Vec<Image>,
    }

    #[derive(serde::Deserialize)]
    struct Image {
        url: String,
        width: Option<u32>,
    }

    pub(super) fn enabled<R>(app: &tauri::AppHandle<R>) -> bool
    where
        R: tauri::Runtime,
    {
        let store = app
            .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
            .build();

        matches!(
            store.get("Spotify#Enabled"),
            Some(serde_json::Value::Bool(true))
        )
    }

    pub(super) async fn currently_playing() -> Result<
        Option<ipc::types::CurrentlyPlaying>,
        Box<dyn std::error::Error + Send + Sync + 'static>,
    > {
        let client = oauth_bridge::Client::new(crate::commands::SPOTIFY_OAUTH_BRIDGE_URL);
        let access_token = client
            .get_access_token("spotify", SCOPE, || async {
                let client = bakery::Client::new(crate::commands::BAKERY_URL, 0);
                Ok(format!(
                    "{}={}",
                    crate::commands::COOKIE_NAME,
                    client.get_value(crate::commands::COOKIE_NAME).await?
                ))
            })
            .await?;

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

        let request = hyper::Request::get(format!(
            "{}/me/player/currently-playing",
            crate::commands::SPOTIFY_API_URL
        ))
        .header(
            hyper::header::AUTHORIZATION,
            format!("Bearer {}", access_token),
        )
        .body(hyper::Body::empty())?;

        let (status, bytes) = tokio::time::timeout(REQUEST_TIMEOUT, async {
            let response = client.request(request).await?;
            let status = response.status();
            let bytes = hyper::body::to_bytes(response.into_body()).await?;
            Ok::<_, Box<dyn std::error::Error + Send + Sync + 'static>>((status, bytes))
        })
        .await??;

        // https://developer.spotify.com/documentation/web-api/concepts/api-calls
        if status == hyper::StatusCode::NO_CONTENT {
            return Ok(None);
        }
        if !status.is_success() {
            return Err(format!(
                "failed to get currently playing track: status {}: {}",
                status,
                String::from_utf8_lossy(&bytes)
            )
            .into());
        }

        let currently_playing: CurrentlyPlaying = serde_json::from_slice(&bytes)?;
        Ok(currently_playing.item.map(|track| {
            // https://developer.spotify.com/documentation/web-api/reference/get-the-users-currently-playing-track returns album images widest first
            // src/pages/Spotify.tsx renders the artwork at 72px (144px at 2x scale)
            let image_url = track
                .album
                .images
                .iter()
                .rev()
                .find(|image| image.width.is_some_and(|width| width >= 144))
                .or_else(|| track.album.images.first())
                .map(|image| image.url.clone());

            ipc::types::CurrentlyPlaying {
                track: track.name,
                artists: track
                    .artists
                    .into_iter()
                    .map(|artist| artist.name)
                    .collect::<Vec<_>>()
                    .join(", "),
                album: track.album.name,
                image_url,
                progress_milliseconds: currently_playing.progress_ms,
                duration_milliseconds: track.duration_ms,
                is_playing: currently_playing.is_playing,
            }
        }))
    }
}
