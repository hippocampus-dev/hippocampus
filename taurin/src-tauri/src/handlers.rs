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

#[cfg(feature = "cuda")]
pub fn voice_input_handler<R>(
    value: tauri_plugin_global_shortcut::Shortcut,
    revise: bool,
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
        if shortcut != &value {
            return;
        }

        let app = app.clone();
        match event.state {
            tauri_plugin_global_shortcut::ShortcutState::Pressed => {
                tauri::async_runtime::spawn(async move {
                    if let Err(e) =
                        crate::commands::voice_input::internal::start_recording(app, revise).await
                    {
                        eprintln!("Error starting voice input: {e}");
                    }
                });
            }
            tauri_plugin_global_shortcut::ShortcutState::Released => {
                tauri::async_runtime::spawn(async move {
                    if let Err(e) =
                        crate::commands::voice_input::internal::stop_recording_and_transcribe(
                            app, revise,
                        )
                        .await
                    {
                        eprintln!("Error stopping voice input: {e}");
                    }
                });
            }
        }
    })
}

#[cfg(feature = "cuda")]
pub fn voice_input_review_handler<R>(
    value: tauri_plugin_global_shortcut::Shortcut,
    accept: bool,
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
        if shortcut != &value {
            return;
        }

        // XGrabKey keeps the keyboard grabbed from Pressed until Released, so keys typed before Released go to taurin (https://xorg.freedesktop.org/archive/current/doc/libX11/libX11/libX11.html#XGrabKey)
        if let tauri_plugin_global_shortcut::ShortcutState::Released = event.state {
            let app = app.clone();
            tauri::async_runtime::spawn(async move {
                if let Err(e) =
                    crate::commands::voice_input::internal::finish_review(app, accept).await
                {
                    eprintln!("Error finishing voice input review: {e}");
                }
            });
        }
    })
}
