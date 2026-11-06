const COMMANDS: &[&str] = &["categorize", "parse_intent"];

fn main() {
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
