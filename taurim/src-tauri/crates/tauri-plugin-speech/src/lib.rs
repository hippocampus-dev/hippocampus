use tauri::Manager;

struct Speech<R: tauri::Runtime>(tauri::plugin::PluginHandle<R>);

#[tauri::command]
async fn start_listening<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    app.state::<Speech<R>>()
        .0
        .run_mobile_plugin("startListening", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn stop_listening<R: tauri::Runtime>(app: tauri::AppHandle<R>) -> Result<(), String> {
    app.state::<Speech<R>>()
        .0
        .run_mobile_plugin("stopListening", ())
        .map_err(|e| e.to_string())
}

pub fn init<R: tauri::Runtime>() -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("speech")
        .invoke_handler(tauri::generate_handler![start_listening, stop_listening])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.speech", "SpeechPlugin")
                .expect("failed to register speech plugin");
            app.manage(Speech(handle));
            Ok(())
        })
        .build()
}
