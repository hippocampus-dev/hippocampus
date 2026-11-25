#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::Manager;
use tauri_plugin_global_shortcut::GlobalShortcutExt;
use tauri_plugin_store::StoreExt;

const AUTO_UPDATE_MAX_RETRIES: usize = 60;
// tauri-releases runs in the local minikube, which is still starting when taurin autostarts at login
const AUTO_UPDATE_RETRY_INTERVAL: std::time::Duration = std::time::Duration::from_secs(60);

mod commands;
mod handlers;
mod native_messaging;

#[derive(Debug)]
pub struct Cancellation {
    sender: tokio::sync::watch::Sender<u64>,
}

impl Default for Cancellation {
    fn default() -> Self {
        Cancellation {
            sender: tokio::sync::watch::channel(0).0,
        }
    }
}

impl Cancellation {
    pub fn begin(&self) -> tokio::sync::watch::Receiver<u64> {
        self.sender.subscribe()
    }

    pub fn cancel(&self) {
        self.sender.send_modify(|generation| *generation += 1);
    }
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct RealtimeTranslation {
    volume_toggle: std::sync::Arc<std::sync::atomic::AtomicBool>,
    #[serde(skip)]
    cancellation: std::sync::Arc<Cancellation>,
    #[serde(default = "default_language")]
    language: String,
}

fn default_language() -> String {
    "en-US".to_string()
}

#[cfg(feature = "cuda")]
#[derive(Clone)]
pub struct VoiceInput {
    recording: std::sync::Arc<std::sync::atomic::AtomicBool>,
    capture_stop: std::sync::Arc<(std::sync::Mutex<bool>, std::sync::Condvar)>,
    model: std::sync::Arc<tokio::sync::Mutex<Option<whisper_rs::WhisperContext>>>,
    model_name: String,
    language: String,
    prompt: String,
    rewrite: bool,
    revise: bool,
    processing: bool,
    candidate: Option<String>,
    rewriter: std::sync::Arc<
        tokio::sync::Mutex<Option<crate::commands::voice_input::internal::Rewriter>>,
    >,
    rewrite_downloading: std::sync::Arc<std::sync::atomic::AtomicBool>,
    rewrite_download_cancellation: std::sync::Arc<Cancellation>,
    audio_buffer: std::sync::Arc<std::sync::Mutex<Vec<f32>>>,
    device_name: Option<String>,
    capture_notify: std::sync::Arc<tokio::sync::Notify>,
    downloading: std::sync::Arc<std::sync::atomic::AtomicBool>,
    download_cancellation: std::sync::Arc<Cancellation>,
}

#[cfg(feature = "cuda")]
impl std::fmt::Debug for VoiceInput {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("VoiceInput")
            .field("recording", &self.recording)
            .field("model_name", &self.model_name)
            .field("device_name", &self.device_name)
            .finish()
    }
}

#[cfg(feature = "cuda")]
impl Default for VoiceInput {
    fn default() -> Self {
        Self {
            recording: std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false)),
            capture_stop: std::sync::Arc::new((
                std::sync::Mutex::new(false),
                std::sync::Condvar::new(),
            )),
            model: std::sync::Arc::new(tokio::sync::Mutex::new(None)),
            model_name: "base".to_string(),
            language: "auto".to_string(),
            prompt: String::new(),
            rewrite: false,
            revise: false,
            processing: false,
            candidate: None,
            rewriter: std::sync::Arc::new(tokio::sync::Mutex::new(None)),
            rewrite_downloading: std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false)),
            rewrite_download_cancellation: std::sync::Arc::new(Cancellation::default()),
            audio_buffer: std::sync::Arc::new(std::sync::Mutex::new(Vec::new())),
            device_name: None,
            capture_notify: std::sync::Arc::new(tokio::sync::Notify::new()),
            downloading: std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false)),
            download_cancellation: std::sync::Arc::new(Cancellation::default()),
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct Bake {
    cancellation: std::sync::Arc<Cancellation>,
}

#[derive(Clone, Debug, Default)]
pub struct ExplainMonitors {
    cancellation: std::sync::Arc<Cancellation>,
}

#[derive(Clone, Debug, Default)]
pub struct CreateGitHubRepositories {
    cancellation: std::sync::Arc<Cancellation>,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AppState {
    realtime_translation: RealtimeTranslation,
    #[cfg(feature = "cuda")]
    #[serde(skip)]
    voice_input: VoiceInput,
    #[serde(skip)]
    bake: Bake,
    #[serde(skip)]
    explain_monitors: ExplainMonitors,
    #[serde(skip)]
    create_github_repositories: CreateGitHubRepositories,
    #[serde(skip)]
    native_messaging: Option<crate::native_messaging::Bridge>,
}

fn main() {
    if crate::native_messaging::is_native_messaging(std::env::args().skip(1)) {
        if let Err(e) = crate::native_messaging::run() {
            eprintln!("Error occurred while handling native messaging: {e}");
            std::process::exit(1);
        }
        return;
    }

    let builder = tauri::Builder::default()
        .plugin(
            tauri_plugin_window_state::Builder::new()
                .with_denylist(&["voice-indicator", "tmux-monitor", "spotify"])
                .build(),
        )
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            None,
        ))
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_os::init())
        .setup(|app| {
            let store = app
                .handle()
                .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
                .build();

            #[cfg(desktop)]
            {
                // Both builds share Chrome's native messaging host name dev.kaidotio.taurin
                if !cfg!(debug_assertions)
                    && let Err(e) = crate::native_messaging::register_host(app.handle())
                {
                    eprintln!("Error occurred while registering native messaging host: {e}");
                }

                let auto_update = match store.get("Enable Auto Update") {
                    Some(serde_json::Value::Bool(auto_update)) => auto_update,
                    _ => false,
                };

                // `tauri dev` runs target/debug/taurin, which tauri-plugin-updater replaces with the release on Linux
                if auto_update && !cfg!(debug_assertions) {
                    let handle = app.app_handle().clone();
                    tauri::async_runtime::spawn(async move {
                        let mut retries = 0;
                        while let Err(e) = crate::commands::auto_update::update(&handle).await {
                            eprintln!("Error occurred while updating: {e}");
                            // Retrying without a token would open the browser for sign-in again
                            if retries >= AUTO_UPDATE_MAX_RETRIES
                                || crate::commands::auto_update::requires_authorization(&handle)
                                    && handle
                                        .store_builder(
                                            crate::commands::settings::SETTINGS_FILE_NAME,
                                        )
                                        .build()
                                        .get("Auto Update#Token")
                                        .is_none()
                            {
                                break;
                            }
                            retries += 1;
                            tokio::time::sleep(AUTO_UPDATE_RETRY_INTERVAL).await;
                        }
                    });
                }

                let default_hot_key = tauri_plugin_global_shortcut::Shortcut::new(
                    Some(tauri_plugin_global_shortcut::Modifiers::ALT),
                    tauri_plugin_global_shortcut::Code::Space,
                );
                let hot_key = match store.get("Hot-key") {
                    Some(v) => serde_json::from_value(v).unwrap_or(default_hot_key),
                    _ => default_hot_key,
                };

                app.global_shortcut()
                    .on_shortcut(hot_key, crate::handlers::hot_key_handler(hot_key))?;

                let realtime_translation_language = match store.get("Realtime Translation#Language")
                {
                    Some(v) => serde_json::from_value(v).unwrap_or_else(|_| default_language()),
                    _ => default_language(),
                };

                let mut state = AppState::default();
                state.realtime_translation.language = realtime_translation_language;
                match crate::native_messaging::Bridge::start() {
                    Ok(bridge) => state.native_messaging = Some(bridge),
                    Err(e) => {
                        eprintln!("Error occurred while starting native messaging bridge: {e}")
                    }
                }

                #[cfg(feature = "cuda")]
                {
                    let voice_input_model = match store.get("Voice Input#Model") {
                        Some(v) => serde_json::from_value(v).unwrap_or_else(|_| "base".to_string()),
                        _ => "base".to_string(),
                    };

                    let voice_input_language = match store.get("Voice Input#Language") {
                        Some(v) => serde_json::from_value(v).unwrap_or_else(|_| "auto".to_string()),
                        _ => "auto".to_string(),
                    };

                    let voice_input_prompt = match store.get("Voice Input#Prompt") {
                        Some(v) => serde_json::from_value(v).unwrap_or_default(),
                        _ => String::new(),
                    };

                    let voice_input_rewrite = match store.get("Voice Input#Rewrite") {
                        Some(serde_json::Value::Bool(rewrite)) => rewrite,
                        _ => false,
                    };

                    let voice_input_device: Option<String> = match store.get("Voice Input#Device") {
                        Some(v) => serde_json::from_value::<Option<String>>(v)
                            .unwrap_or(None)
                            .filter(|name| name != "Default"),
                        _ => None,
                    };

                    state.voice_input.model_name = voice_input_model;
                    state.voice_input.language = voice_input_language;
                    state.voice_input.prompt = voice_input_prompt;
                    state.voice_input.rewrite = voice_input_rewrite;
                    state.voice_input.device_name = voice_input_device;
                }

                app.manage(tokio::sync::Mutex::new(state));

                #[cfg(feature = "cuda")]
                {
                    let default_voice_input_shortcut = tauri_plugin_global_shortcut::Shortcut::new(
                        Some(
                            tauri_plugin_global_shortcut::Modifiers::ALT
                                | tauri_plugin_global_shortcut::Modifiers::SHIFT,
                        ),
                        tauri_plugin_global_shortcut::Code::Space,
                    );
                    let voice_input_shortcut = match store.get("Voice Input#Shortcut") {
                        Some(v) => {
                            serde_json::from_value(v).unwrap_or(default_voice_input_shortcut)
                        }
                        _ => default_voice_input_shortcut,
                    };

                    crate::commands::voice_input::internal::load_whisper_model(
                        app.handle().clone(),
                    );
                    crate::commands::voice_input::internal::load_rewriter(app.handle().clone());

                    app.global_shortcut().on_shortcut(
                        voice_input_shortcut,
                        crate::handlers::voice_input_handler(voice_input_shortcut, false),
                    )?;
                }

                let tmux_monitor_enabled = match store.get("tmux Monitor#Enabled") {
                    Some(serde_json::Value::Bool(enabled)) => enabled,
                    _ => false,
                };

                crate::commands::tmux_monitor::apply_overlay_visibility(
                    app.handle(),
                    tmux_monitor_enabled,
                );

                let spotify_enabled = match store.get("Spotify#Enabled") {
                    Some(serde_json::Value::Bool(enabled)) => enabled,
                    _ => false,
                };

                crate::commands::spotify::apply_overlay_visibility(app.handle(), spotify_enabled);
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            crate::commands::settings::get_settings,
            crate::commands::settings::handle_auto_start,
            crate::commands::settings::handle_auto_update,
            crate::commands::settings::handle_hot_key,
            crate::commands::settings::handle_realtime_translation_language,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_shortcut,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_model,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_language,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_prompt,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_rewrite,
            #[cfg(feature = "cuda")]
            crate::commands::settings::handle_voice_input_device,
            crate::commands::settings::handle_tmux_monitor_enabled,
            crate::commands::settings::handle_tmux_monitor_filter,
            crate::commands::settings::handle_spotify_enabled,
            crate::commands::auto_update::check_for_updates,
            crate::commands::bake,
            crate::commands::cancel_bake,
            crate::commands::explain_monitors,
            crate::commands::cancel_explain_monitors,
            crate::commands::save_window_state,
            crate::commands::ping_extension,
            crate::commands::create_github_repositories::read_github_repositories,
            crate::commands::create_github_repositories::create_github_repositories,
            crate::commands::create_github_repositories::cancel_create_github_repositories,
            crate::commands::create_github_repositories::save_github_repositories_result,
            crate::commands::realtime_translation::start_realtime_translation,
            crate::commands::realtime_translation::stop_realtime_translation,
            crate::commands::spotify::get_spotify_currently_playing,
            crate::commands::spotify::has_spotify_token,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::list_audio_input_devices,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::download_whisper_model,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::cancel_download_whisper_model,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::get_whisper_model_status,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::download_rewrite_model,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::cancel_download_rewrite_model,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::get_rewrite_model_status,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::start_voice_input,
            #[cfg(feature = "cuda")]
            crate::commands::voice_input::stop_voice_input,
            crate::commands::tmux_monitor::list_tmux_windows,
            crate::commands::overlay::save_overlay_position,
            crate::commands::overlay::reset_overlay_positions,
        ]);

    let app = builder
        .build(tauri::generate_context!())
        .expect("error while building tauri application");
    app.run(|app_handle, event| {
        if matches!(event, tauri::RunEvent::Exit)
            && let Some(state) = app_handle.try_state::<tokio::sync::Mutex<AppState>>()
            && let Some(bridge) = state.blocking_lock().native_messaging.take()
        {
            bridge.shutdown();
        }
    });
}
