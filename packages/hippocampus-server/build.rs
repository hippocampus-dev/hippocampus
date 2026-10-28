fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    tonic_build::configure().build_server(true).compile(
        &[
            "../../proto/helloworld.proto",
            "../../proto/hippocampus.proto",
        ],
        &["../../proto", "../../proto/googleapis"],
    )?;
    Ok(())
}
