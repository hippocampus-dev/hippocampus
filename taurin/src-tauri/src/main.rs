#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::Manager;
use tauri_plugin_dialog::DialogExt;
use tauri_plugin_global_shortcut::GlobalShortcutExt;
use tauri_plugin_store::StoreExt;
use tauri_plugin_updater::UpdaterExt;

mod commands;
mod handlers;

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct RealtimeTranslation {
    volume_toggle: std::sync::Arc<std::sync::atomic::AtomicBool>,
    #[serde(skip)]
    stop_notify: std::sync::Arc<tokio::sync::Notify>,
    #[serde(default = "default_language")]
    language: String,
}

fn default_language() -> String {
    "en-US".to_string()
}

#[derive(Clone)]
pub struct VoiceInput {
    recording: std::sync::Arc<std::sync::atomic::AtomicBool>,
    capture_stop: std::sync::Arc<(std::sync::Mutex<bool>, std::sync::Condvar)>,
    model: std::sync::Arc<tokio::sync::Mutex<Option<whisper_rs::WhisperContext>>>,
    model_name: String,
    language: String,
    audio_buffer: std::sync::Arc<std::sync::Mutex<Vec<f32>>>,
    device_name: Option<String>,
    capture_notify: std::sync::Arc<tokio::sync::Notify>,
}

impl std::fmt::Debug for VoiceInput {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("VoiceInput")
            .field("recording", &self.recording)
            .field("model_name", &self.model_name)
            .field("device_name", &self.device_name)
            .finish()
    }
}

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
            audio_buffer: std::sync::Arc::new(std::sync::Mutex::new(Vec::new())),
            device_name: None,
            capture_notify: std::sync::Arc::new(tokio::sync::Notify::new()),
        }
    }
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AppState {
    realtime_translation: RealtimeTranslation,
    #[serde(skip)]
    voice_input: VoiceInput,
}

fn main() {
    let builder = tauri::Builder::default()
        .plugin(
            tauri_plugin_window_state::Builder::new()
                .with_denylist(&["voice-indicator"])
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
            let store = app.handle().store_builder("settings.bin").build();

            #[cfg(desktop)]
            {
                let auto_update = match store.get("Enable Auto Update") {
                    Some(serde_json::Value::Bool(auto_update)) => auto_update,
                    _ => false,
                };

                if auto_update {
                    let handle = app.app_handle().clone();
                    tauri::async_runtime::spawn(async move {
                        if let Ok(updater) = handle.updater() {
                            if let Ok(Some(update)) = updater.check().await {
                                if let Err(e) = update.download_and_install(|_, _| {}, || {}).await
                                {
                                    eprintln!("Error occurred while downloading: {}", e);
                                    return;
                                }

                                #[cfg(target_os = "linux")]
                                if let Some(appimage) = handle.env().appimage {
                                    use std::os::unix::fs::PermissionsExt;
                                    if let Err(e) = std::fs::set_permissions(
                                        appimage,
                                        std::fs::Permissions::from_mode(0o755),
                                    ) {
                                        eprintln!(
                                            "Error occurred while setting permissions: {}",
                                            e
                                        );
                                        return;
                                    }
                                }

                                handle
                                    .dialog()
                                    .message("Update downloaded and installed successfully!")
                                    .title("Auto Update")
                                    .buttons(
                                        tauri_plugin_dialog::MessageDialogButtons::OkCancelCustom(
                                            "Restart".to_string(),
                                            "Later".to_string(),
                                        ),
                                    )
                                    .show(move |result| match result {
                                        true => handle.restart(),
                                        false => {}
                                    });
                            }
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

                let default_voice_input_shortcut = tauri_plugin_global_shortcut::Shortcut::new(
                    Some(
                        tauri_plugin_global_shortcut::Modifiers::ALT
                            | tauri_plugin_global_shortcut::Modifiers::SHIFT,
                    ),
                    tauri_plugin_global_shortcut::Code::Space,
                );
                let voice_input_shortcut = match store.get("Voice Input#Shortcut") {
                    Some(v) => serde_json::from_value(v).unwrap_or(default_voice_input_shortcut),
                    _ => default_voice_input_shortcut,
                };

                let voice_input_model = match store.get("Voice Input#Model") {
                    Some(v) => serde_json::from_value(v).unwrap_or_else(|_| "base".to_string()),
                    _ => "base".to_string(),
                };

                let voice_input_language = match store.get("Voice Input#Language") {
                    Some(v) => serde_json::from_value(v).unwrap_or_else(|_| "auto".to_string()),
                    _ => "auto".to_string(),
                };

                let voice_input_device: Option<String> = match store.get("Voice Input#Device") {
                    Some(v) => serde_json::from_value::<Option<String>>(v)
                        .unwrap_or(None)
                        .filter(|name| name != "Default"),
                    _ => None,
                };

                let mut state = AppState::default();
                state.realtime_translation.language = realtime_translation_language;
                state.voice_input.model_name = voice_input_model;
                state.voice_input.language = voice_input_language;
                state.voice_input.device_name = voice_input_device;
                app.manage(tokio::sync::Mutex::new(state));

                app.global_shortcut().on_shortcut(
                    voice_input_shortcut,
                    crate::handlers::voice_input_handler(voice_input_shortcut),
                )?;
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            crate::commands::settings::get_settings,
            crate::commands::settings::handle_auto_start,
            crate::commands::settings::handle_auto_update,
            crate::commands::settings::handle_hot_key,
            crate::commands::settings::handle_realtime_translation_language,
            crate::commands::settings::handle_voice_input_shortcut,
            crate::commands::settings::handle_voice_input_model,
            crate::commands::settings::handle_voice_input_language,
            crate::commands::settings::handle_voice_input_device,
            crate::commands::bake,
            crate::commands::explain_monitors,
            crate::commands::realtime_translation::start_realtime_translation,
            crate::commands::realtime_translation::stop_realtime_translation,
            crate::commands::voice_input::list_audio_input_devices,
            crate::commands::voice_input::download_whisper_model,
            crate::commands::voice_input::get_whisper_model_status,
        ]);

    builder
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
