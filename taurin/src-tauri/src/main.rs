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
    #[serde(default = "default_language")]
    language: String,
}

fn default_language() -> String {
    "en-US".to_string()
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AppState {
    realtime_translation: RealtimeTranslation,
}

fn main() {
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_window_state::Builder::new().build())
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

                let mut state = AppState::default();
                state.realtime_translation.language = realtime_translation_language;
                app.manage(tokio::sync::Mutex::new(state));
            }

            #[cfg(debug_assertions)]
            {
                // let window = app.get_webview_window("main").unwrap();
                // window.open_devtools();
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            crate::commands::settings::get_settings,
            crate::commands::settings::handle_auto_start,
            crate::commands::settings::handle_auto_update,
            crate::commands::settings::handle_hot_key,
            crate::commands::settings::handle_realtime_translation_language,
            crate::commands::bake,
            crate::commands::explain_monitors,
            crate::commands::realtime_translation::start_realtime_translation,
            crate::commands::realtime_translation::stop_realtime_translation,
        ]);

    builder
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
