use tauri::Manager;
use tauri_plugin_store::StoreExt;

// #{pane_title} is free-form text the running application sets, so the fields are separated by a control character rather than a tab
const FIELD_SEPARATOR: char = '\x1f';
const FIELD_COUNT: usize = 4;
// #{window_silence_flag} is raised on the window, so the active pane of each window stands for the whole window
const WINDOW_FORMAT: &str =
    "#{session_name}\x1f#{window_index}\x1f#{window_silence_flag}\x1f#{pane_title}";

const OVERLAY_WIDTH: f64 = 400.0;
const OVERLAY_MARGIN: f64 = 16.0;

#[tauri::command]
#[ipc::estringify]
pub async fn list_tmux_windows(
    app_handle: tauri::AppHandle,
) -> Result<ipc::types::TmuxWindows, Box<dyn std::error::Error + Send + Sync + 'static>> {
    tokio::task::spawn_blocking(move || {
        let store = app_handle
            .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
            .build();

        let filter = match store.get("tmux Monitor#Filter") {
            Some(v) => serde_json::from_value(v)?,
            _ => String::new(),
        };

        internal::list_windows(&filter)
    })
    .await?
}

#[tauri::command]
#[ipc::estringify]
pub async fn save_tmux_monitor_position(
    app_handle: tauri::AppHandle,
    position: tauri::PhysicalPosition<i32>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let store = app_handle
        .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
        .build();
    store.set("tmux Monitor#Position", serde_json::to_value(position)?);
    store.save()?;

    Ok(())
}

pub fn apply_overlay_visibility<R>(app: &tauri::AppHandle<R>, enabled: bool)
where
    R: tauri::Runtime,
{
    if let Some(overlay) = app.get_webview_window("tmux-monitor") {
        if !enabled {
            let _ = overlay.hide();
            return;
        }

        match internal::saved_position(app) {
            Some(position) => {
                let _ = overlay.set_position(position);
            }
            None => {
                let monitor = overlay
                    .current_monitor()
                    .ok()
                    .flatten()
                    .or_else(|| overlay.primary_monitor().ok().flatten());

                if let Some(monitor) = monitor {
                    let scale = monitor.scale_factor();
                    let screen_width = monitor.size().width as f64 / scale;

                    let _ = overlay.set_position(tauri::LogicalPosition::new(
                        screen_width - OVERLAY_WIDTH - OVERLAY_MARGIN,
                        OVERLAY_MARGIN,
                    ));
                }
            }
        }

        let _ = overlay.show();
    }
}

mod internal {
    use super::*;

    pub(super) fn list_windows(
        filter: &str,
    ) -> Result<ipc::types::TmuxWindows, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let output = std::process::Command::new("tmux")
            .args([
                "list-panes",
                "-a",
                "-f",
                "#{pane_active}",
                "-F",
                WINDOW_FORMAT,
            ])
            .output()?;

        // tmux exits 1 while no server is running, which is the ordinary state until the first session starts
        if !output.status.success() {
            return Ok(Vec::new());
        }

        let matcher = if filter.is_empty() {
            None
        } else {
            Some(regex::Regex::new(filter)?)
        };

        Ok(String::from_utf8_lossy(&output.stdout)
            .lines()
            .filter_map(|line| {
                let mut fields = line.splitn(FIELD_COUNT, FIELD_SEPARATOR);
                Some(ipc::types::TmuxWindow {
                    session_name: fields.next()?.to_string(),
                    window_index: fields.next()?.to_string(),
                    silent: fields.next()? == "1",
                    title: fields.next()?.to_string(),
                })
            })
            .filter(|window| {
                matcher.as_ref().is_none_or(|matcher| {
                    matcher.is_match(&format!(
                        "{}:{} {}",
                        window.session_name, window.window_index, window.title
                    ))
                })
            })
            .collect())
    }

    pub(super) fn saved_position<R>(
        app: &tauri::AppHandle<R>,
    ) -> Option<tauri::PhysicalPosition<i32>>
    where
        R: tauri::Runtime,
    {
        let store = app
            .store_builder(crate::commands::settings::SETTINGS_FILE_NAME)
            .build();

        serde_json::from_value(store.get("tmux Monitor#Position")?).ok()
    }
}
