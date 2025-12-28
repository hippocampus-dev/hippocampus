use tauri::Manager;

pub fn hot_key_handler<R>(
    value: tauri_plugin_global_shortcut::Shortcut,
) -> Box<
    dyn Fn(
            &tauri::AppHandle<R>,
            &tauri_plugin_global_shortcut::Shortcut,
            tauri_plugin_global_shortcut::ShortcutEvent,
        ) + Send
        + Sync
        + 'static,
>
where
    R: tauri::Runtime,
{
    Box::new(move |app, shortcut, event| {
        let window = app.get_webview_window("main").unwrap();
        if shortcut == &value {
            match event.state {
                tauri_plugin_global_shortcut::ShortcutState::Pressed => {
                    if window.is_visible().unwrap() {
                        window.hide().unwrap();
                    } else {
                        window.show().unwrap();
                        window.set_focus().unwrap();
                    }
                }
                _ => {}
            }
        }
    })
}
