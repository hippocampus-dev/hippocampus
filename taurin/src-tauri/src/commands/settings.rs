use tauri::Manager;
use tauri_plugin_autostart::ManagerExt;
use tauri_plugin_global_shortcut::GlobalShortcutExt;
use tauri_plugin_store::StoreExt;

const SETTINGS_FILE_NAME: &str = "settings.bin";

#[tauri::command]
#[ipc::estringify]
pub async fn get_settings(
    app_handle: tauri::AppHandle,
) -> Result<ipc::types::Settings, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build();

    let mut default_settings = vec![];

    #[cfg(desktop)]
    {
        let mut desktop_settings = vec![
            ipc::types::Setting {
                current_value: match store.get("Auto Start") {
                    Some(serde_json::Value::Bool(auto_start)) => {
                        ipc::types::Value::Bool(auto_start)
                    }
                    _ => ipc::types::Value::Bool(false),
                },
                kind: ipc::types::Kind::Switch,
                handler_command: "handle_auto_start".to_string(),
            },
            ipc::types::Setting {
                current_value: match store.get("Enable Auto Update") {
                    Some(serde_json::Value::Bool(auto_update)) => {
                        ipc::types::Value::Bool(auto_update)
                    }
                    _ => ipc::types::Value::Bool(false),
                },
                kind: ipc::types::Kind::Switch,
                handler_command: "handle_auto_update".to_string(),
            },
            ipc::types::Setting {
                current_value: store
                    .get("Hot-key")
                    .and_then(|v| serde_json::from_value(v).ok())
                    .map(ipc::types::Value::Shortcut)
                    .unwrap_or_else(|| {
                        ipc::types::Value::Shortcut(tauri_plugin_global_shortcut::Shortcut::new(
                            Some(tauri_plugin_global_shortcut::Modifiers::ALT),
                            tauri_plugin_global_shortcut::Code::Space,
                        ))
                    }),
                kind: ipc::types::Kind::Shortcut,
                handler_command: "handle_hot_key".to_string(),
            },
        ];

        default_settings.append(&mut desktop_settings);
    }

    default_settings.push(ipc::types::Setting {
        current_value: match store.get("Realtime Translation#Language") {
            Some(v) => ipc::types::Value::String(serde_json::from_value(v)?),
            _ => ipc::types::Value::String("en-US".to_string()),
        },
        kind: ipc::types::Kind::Input,
        handler_command: "handle_realtime_translation_language".to_string(),
    });

    Ok(default_settings)
}

#[tauri::command]
#[ipc::estringify]
pub async fn handle_auto_start(
    app_handle: tauri::AppHandle,
    value: bool,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build();
    store.set("Auto Start", value);
    store.save()?;

    let autostart_manager = app_handle.autolaunch();
    if value {
        let _ = autostart_manager.enable();
    } else {
        let _ = autostart_manager.disable();
    }

    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn handle_auto_update(
    app_handle: tauri::AppHandle,
    value: bool,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build();
    store.set("Enable Auto Update", value);
    store.save()?;

    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn handle_hot_key(
    app_handle: tauri::AppHandle,
    value: tauri_plugin_global_shortcut::Shortcut,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build();

    if let Some(v) = store.get("Hot-key") {
        let old_value: tauri_plugin_global_shortcut::Shortcut = serde_json::from_value(v)?;
        app_handle.global_shortcut().unregister(old_value)?;
    }

    store.set("Hot-key", serde_json::to_value(value)?);
    store.save()?;
    app_handle
        .global_shortcut()
        .on_shortcut(value, crate::handlers::hot_key_handler(value))?;

    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn handle_realtime_translation_language(
    app_handle: tauri::AppHandle,
    value: String,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle.store_builder(SETTINGS_FILE_NAME).build();
    store.set(
        "Realtime Translation#Language",
        serde_json::to_value(value.clone())?,
    );
    store.save()?;

    let state = app_handle.state::<tokio::sync::Mutex<crate::AppState>>();
    let mut state = state.lock().await;
    state.realtime_translation.language = value;

    Ok(())
}
