const COMMANDS: &[&str] = &["start_listening", "stop_listening", "register_listener", "remove_listener", "request_permissions"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
