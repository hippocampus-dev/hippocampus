#[cfg(target_os = "android")]
const AUTO_UPDATE_MAX_RETRIES: usize = 60;
#[cfg(target_os = "android")]
const AUTO_UPDATE_RETRY_INTERVAL: std::time::Duration = std::time::Duration::from_secs(60);

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default();

    #[cfg(target_os = "android")]
    let builder = builder
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_auto_update::init())
        .plugin(tauri_plugin_gemini::init())
        .plugin(tauri_plugin_timer::init())
        .plugin(tauri_plugin_speech::init())
        .plugin(tauri_plugin_wake_on_lan::init())
        .setup(|app| {
            if !cfg!(debug_assertions) {
                let handle = tauri::Manager::app_handle(app).clone();
                tauri::async_runtime::spawn(async move {
                    let mut retries = 0;
                    while let Err(e) = tauri_plugin_auto_update::update(&handle).await {
                        eprintln!("Error occurred while updating: {e}");
                        if retries >= AUTO_UPDATE_MAX_RETRIES
                            // Retrying without a token would open the browser for sign-in again
                            || tauri_plugin_auto_update::requires_authorization()
                                && !tauri_plugin_auto_update::has_token(&handle)
                            // Retrying after a signature mismatch would download the same APK again
                            || e.is::<minisign_verify::Error>()
                        {
                            break;
                        }
                        retries += 1;
                        tokio::time::sleep(AUTO_UPDATE_RETRY_INTERVAL).await;
                    }
                });
            }
            Ok(())
        });

    builder
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
