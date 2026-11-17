const SCOPE: &str = "user-read-currently-playing";

#[tauri::command]
pub async fn get_spotify_currently_playing(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<String, ipc::CommandError> {
    let mut cancelled = state
        .lock()
        .await
        .get_spotify_currently_playing
        .cancellation
        .begin();

    tokio::select! {
        currently_playing = internal::currently_playing() => Ok(currently_playing?),
        Ok(()) = cancelled.changed() => Err(ipc::CommandError::Cancelled),
    }
}

#[tauri::command]
pub async fn cancel_get_spotify_currently_playing(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), ipc::CommandError> {
    state
        .lock()
        .await
        .get_spotify_currently_playing
        .cancellation
        .cancel();
    Ok(())
}

mod internal {
    use super::*;

    #[derive(serde::Deserialize)]
    struct CurrentlyPlaying {
        is_playing: bool,
        item: Option<Track>,
    }

    #[derive(serde::Deserialize)]
    struct Track {
        name: String,
        artists: Vec<Artist>,
    }

    #[derive(serde::Deserialize)]
    struct Artist {
        name: String,
    }

    pub(super) async fn currently_playing()
    -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
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

        let response = client.request(request).await?;
        let status = response.status();
        let bytes = hyper::body::to_bytes(response.into_body()).await?;

        // https://developer.spotify.com/documentation/web-api/concepts/api-calls
        if status == hyper::StatusCode::NO_CONTENT {
            return Ok("Nothing is playing".to_string());
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
        match currently_playing.item {
            Some(track) if currently_playing.is_playing => Ok(format!(
                "{} - {}",
                track.name,
                track
                    .artists
                    .into_iter()
                    .map(|artist| artist.name)
                    .collect::<Vec<_>>()
                    .join(", ")
            )),
            _ => Ok("Nothing is playing".to_string()),
        }
    }
}
