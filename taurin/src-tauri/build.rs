fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    tauri_build::build();

    let base_directory = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    ipc_build::export(base_directory.join("../src/ipc/types"))?;

    Ok(())
}
