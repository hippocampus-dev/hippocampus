fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/bpf/connect.bpf.c")
        .build_and_generate(std::path::Path::new("src/bpf/skel.rs"))?;
    Ok(())
}
