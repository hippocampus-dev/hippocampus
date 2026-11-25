use tauri::Manager;
use tauri_plugin_store::StoreExt;

const MARGIN: f64 = 16.0;

enum Corner {
    TopLeft,
    TopRight,
}

const OVERLAYS: [(&str, &str, Corner); 2] = [
    ("tmux-monitor", "tmux Monitor#Position", Corner::TopRight),
    ("spotify", "Spotify#Position", Corner::TopLeft),
];

#[tauri::command]
#[ipc::estringify]
pub async fn save_overlay_position(
    app_handle: tauri::AppHandle,
    webview_window: tauri::WebviewWindow,
    position: tauri::PhysicalPosition<i32>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let Some((_, key, _)) = OVERLAYS
        .iter()
        .find(|(label, _, _)| *label == webview_window.label())
    else {
        return Err(format!("{} is not an overlay", webview_window.label()).into());
    };

    let store = app_handle
        .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
        .build();
    store.set(*key, serde_json::to_value(position)?);
    store.save()?;

    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn reset_overlay_positions(
    app_handle: tauri::AppHandle,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle
        .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
        .build();
    for (_, key, _) in &OVERLAYS {
        store.delete(*key);
    }
    store.save()?;

    for (label, _, _) in &OVERLAYS {
        place(&app_handle, label);
    }

    Ok(())
}

pub fn place<R>(app: &tauri::AppHandle<R>, label: &str)
where
    R: tauri::Runtime,
{
    let Some((_, key, corner)) = OVERLAYS.iter().find(|(l, _, _)| *l == label) else {
        return;
    };
    let Some(overlay) = app.get_webview_window(label) else {
        return;
    };

    let store = app
        .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
        .build();
    if let Some(position) = store
        .get(*key)
        .and_then(|v| serde_json::from_value::<tauri::PhysicalPosition<i32>>(v).ok())
    {
        let _ = overlay.set_position(position);
        return;
    }

    match corner {
        Corner::TopLeft => {
            let _ = overlay.set_position(tauri::LogicalPosition::new(MARGIN, MARGIN));
        }
        Corner::TopRight => {
            let monitor = overlay
                .current_monitor()
                .ok()
                .flatten()
                .or_else(|| overlay.primary_monitor().ok().flatten());

            // tao seeds outer_size() with the window's origin until GTK first configures it on screen: https://github.com/tauri-apps/tao/blob/tao-v0.32.8/src/platform_impl/linux/window.rs
            let width = app
                .config()
                .app
                .windows
                .iter()
                .find(|window| window.label == label)
                .map(|window| window.width);

            if let (Some(monitor), Some(width)) = (monitor, width) {
                let scale = monitor.scale_factor();
                let screen_width = monitor.size().width as f64 / scale;

                let _ = overlay.set_position(tauri::LogicalPosition::new(
                    screen_width - width - MARGIN,
                    MARGIN,
                ));
            }
        }
    }
}
