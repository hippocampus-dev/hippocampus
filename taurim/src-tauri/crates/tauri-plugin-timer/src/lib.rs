use serde::{Deserialize, Serialize};
use tauri::{
    plugin::{Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime,
};

#[derive(Debug, Serialize, Deserialize)]
pub struct TimerState {
    pub remaining_seconds: u32,
    pub is_running: bool,
}

#[tauri::command]
async fn start_timer<R: Runtime>(
    app: AppHandle<R>,
    duration_seconds: u32,
) -> Result<(), String> {
    #[derive(Serialize)]
    struct Request {
        duration_seconds: u32,
    }

    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("startTimer", Request { duration_seconds })
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn stop_timer<R: Runtime>(app: AppHandle<R>) -> Result<(), String> {
    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("stopTimer", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_remaining<R: Runtime>(app: AppHandle<R>) -> Result<TimerState, String> {
    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("getRemaining", ())
        .map_err(|e| e.to_string())
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("timer")
        .invoke_handler(tauri::generate_handler![start_timer, stop_timer, get_remaining])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.timer", "TimerPlugin")
                .expect("failed to register timer plugin");
            app.manage(handle);
            Ok(())
        })
        .build()
}
