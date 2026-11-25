use tauri::Manager;

struct Timer<R: tauri::Runtime>(tauri::plugin::PluginHandle<R>);

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct TimerState {
    pub remaining_seconds: u32,
    pub is_running: bool,
}

#[tauri::command]
async fn start_timer<R: tauri::Runtime>(
    app: tauri::AppHandle<R>,
    duration_seconds: u32,
) -> Result<(), String> {
    #[derive(serde::Serialize)]
    struct Request {
        duration_seconds: u32,
    }

    app.state::<Timer<R>>()
        .0
        .run_mobile_plugin("startTimer", Request { duration_seconds })
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn stop_timer<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    app.state::<Timer<R>>()
        .0
        .run_mobile_plugin("stopTimer", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn stop_alarm<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    app.state::<Timer<R>>()
        .0
        .run_mobile_plugin("stopAlarm", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_remaining<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<TimerState, String> {
    app.state::<Timer<R>>()
        .0
        .run_mobile_plugin("getRemaining", ())
        .map_err(|e| e.to_string())
}

pub fn init<R: tauri::Runtime>() -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("timer")
        .invoke_handler(tauri::generate_handler![start_timer, stop_timer, stop_alarm, get_remaining])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.timer", "TimerPlugin")
                .expect("failed to register timer plugin");
            app.manage(Timer(handle));
            Ok(())
        })
        .build()
}
