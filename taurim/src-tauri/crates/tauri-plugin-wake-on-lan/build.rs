const COMMANDS: &[&str] = &["get_schedule", "set_schedule", "wake"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
