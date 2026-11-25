const COMMANDS: &[&str] = &["wait_for_update", "install_update"];

const TAURIN_CONFIGURATION_PATH: &str = "../../../../taurin/src-tauri/tauri.conf.json";

fn main() {
    println!("cargo:rerun-if-changed={TAURIN_CONFIGURATION_PATH}");
    let configuration: serde_json::Value = serde_json::from_str(
        &std::fs::read_to_string(TAURIN_CONFIGURATION_PATH)
            .expect("failed to read taurin's tauri.conf.json"),
    )
    .expect("failed to parse taurin's tauri.conf.json");
    println!(
        "cargo:rustc-env=TAURIN_UPDATER_PUBKEY={}",
        configuration["plugins"]["updater"]["pubkey"]
            .as_str()
            .expect("missing plugins.updater.pubkey in taurin's tauri.conf.json")
    );

    tauri_plugin::mobile::update_android_manifest(
        "AUTO UPDATE PLUGIN",
        "activity",
        r#"<intent-filter android:autoVerify="true">
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="https" android:host="tauri-releases-public.kaidotio.dev" android:path="/taurim/callback" />
</intent-filter>"#
            .to_string(),
    )
    .expect("failed to update AndroidManifest.xml");

    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .build();
}
