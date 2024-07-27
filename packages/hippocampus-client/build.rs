fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    tonic_build::configure()
        .build_client(true)
        .compile(&["../../proto/helloworld.proto"], &["../.."])?;
    Ok(())
}
