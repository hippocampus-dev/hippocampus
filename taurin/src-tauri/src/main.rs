// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::Manager;
use tauri_plugin_autostart::ManagerExt;

// Learn more about Tauri commands at https://tauri.app/v1/guides/features/command
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! You've been greeted from Rust!", name)
}

fn main() {
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            None,
        ))
        .setup(|app| {
            let stores = app
                .app_handle()
                .state::<tauri_plugin_store::StoreCollection<tauri::Wry>>();
            let path = std::path::PathBuf::from("settings.bin");

            tauri_plugin_store::with_store(app.app_handle().clone(), stores, path, |store| {
                let auto_start = match store.get("Enable Auto Start") {
                    Some(serde_json::Value::Bool(auto_start)) => *auto_start,
                    _ => false,
                };

                let autostart_manager = app.autolaunch();
                if auto_start {
                    let _ = autostart_manager.enable();
                } else {
                    let _ = autostart_manager.disable();
                }

                let auto_update = match store.get("Enable Auto Update") {
                    Some(serde_json::Value::Bool(auto_update)) => *auto_update,
                    _ => false,
                };

                if auto_update {
                    app.app_handle()
                        .plugin(tauri_plugin_updater::Builder::new().build())?;
                }

                let hot_key = match store.get("Hot-key") {
                    Some(serde_json::Value::String(hot_key)) => hot_key,
                    _ => "alt+space",
                };

                app.app_handle().plugin(
                    tauri_plugin_global_shortcut::Builder::new()
                        .with_shortcuts([hot_key])
                        .unwrap()
                        .with_handler(|app, shortcut, event| {
                            let window = app.get_webview_window("main").unwrap();
                            if event.state == tauri_plugin_global_shortcut::ShortcutState::Pressed
                                && shortcut.matches(
                                    tauri_plugin_global_shortcut::Modifiers::ALT,
                                    tauri_plugin_global_shortcut::Code::Space,
                                )
                            {
                                if window.is_visible().unwrap() {
                                    window.hide().unwrap();
                                } else {
                                    window.show().unwrap();
                                    window.set_focus().unwrap();
                                }
                            }
                        })
                        .build(),
                )?;

                Ok(())
            })?;

            // #[cfg(debug_assertions)]
            // {
            //     let window = app.get_webview_window("main").unwrap();
            //     window.open_devtools();
            // }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![greet]);

    builder
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
