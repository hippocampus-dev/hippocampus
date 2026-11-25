use tauri::Manager;

struct WakeOnLan<R: tauri::Runtime>(tauri::plugin::PluginHandle<R>);

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct Schedule {
    pub mac_address: String,
    pub hour: u8,
    pub minute: u8,
    pub enabled: bool,
}

#[tauri::command]
async fn get_schedule<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<Schedule, String> {
    app.state::<WakeOnLan<R>>()
        .0
        .run_mobile_plugin("getSchedule", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn set_schedule<R: tauri::Runtime>(
    app: tauri::AppHandle<R>,
    schedule: Schedule,
) -> Result<(), String> {
    app.state::<WakeOnLan<R>>()
        .0
        .run_mobile_plugin("setSchedule", schedule)
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn wake<R: tauri::Runtime>(
    app: tauri::AppHandle<R>,
    mac_address: String,
) -> Result<(), String> {
    #[derive(serde::Serialize)]
    struct Request {
        mac_address: String,
    }

    app.state::<WakeOnLan<R>>()
        .0
        .run_mobile_plugin("wake", Request { mac_address })
        .map_err(|e| e.to_string())
}

pub fn init<R: tauri::Runtime>() -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("wake-on-lan")
        .invoke_handler(tauri::generate_handler![get_schedule, set_schedule, wake])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.wakeonlan", "WakeOnLanPlugin")
                .expect("failed to register wake-on-lan plugin");
            app.manage(WakeOnLan(handle));
            Ok(())
        })
        .build()
}
