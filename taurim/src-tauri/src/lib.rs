#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default();

    #[cfg(target_os = "android")]
    let builder = builder
        .plugin(tauri_plugin_gemini::init())
        .plugin(tauri_plugin_timer::init());

    builder
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
