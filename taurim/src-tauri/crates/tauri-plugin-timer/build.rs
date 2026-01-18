const COMMANDS: &[&str] = &["start_timer", "stop_timer", "get_remaining"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
