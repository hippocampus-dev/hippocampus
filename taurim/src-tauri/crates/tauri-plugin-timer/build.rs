const COMMANDS: &[&str] = &["start_timer", "stop_timer", "stop_alarm", "get_remaining", "register_listener", "remove_listener"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
