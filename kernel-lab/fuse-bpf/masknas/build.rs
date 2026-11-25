fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    #[cfg(feature = "bpf")]
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/bpf/mask.bpf.c")
        .build_and_generate(std::path::Path::new("src/bpf/skel.rs"))?;
    Ok(())
}
