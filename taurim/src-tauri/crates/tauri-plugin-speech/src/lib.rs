use tauri::{
    plugin::{Builder, PluginHandle, TauriPlugin},
    AppHandle, Manager, Runtime,
};

#[tauri::command]
async fn start_listening<R: Runtime>(app: AppHandle<R>) -> Result<(), String> {
    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("startListening", ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn stop_listening<R: Runtime>(app: AppHandle<R>) -> Result<(), String> {
    app.state::<PluginHandle<R>>()
        .run_mobile_plugin("stopListening", ())
        .map_err(|e| e.to_string())
}

pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("speech")
        .invoke_handler(tauri::generate_handler![start_listening, stop_listening])
        .setup(|app, api| {
            let handle = api
                .register_android_plugin("com.plugin.speech", "SpeechPlugin")
                .expect("failed to register speech plugin");
            app.manage(handle);
            Ok(())
        })
        .build()
}
