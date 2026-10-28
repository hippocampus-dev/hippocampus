fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/cpu_usage/cpu_usage.bpf.c")
        .build_and_generate(std::path::Path::new("src/cpu_usage/skel.rs"))?;
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/http/http.bpf.c")
        .build_and_generate(std::path::Path::new("src/http/skel.rs"))?;
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/https/https.bpf.c")
        .build_and_generate(std::path::Path::new("src/https/skel.rs"))?;
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/mysql/mysql.bpf.c")
        .build_and_generate(std::path::Path::new("src/mysql/skel.rs"))?;
    libbpf_cargo::SkeletonBuilder::new()
        .source("src/vfs/vfs.bpf.c")
        .build_and_generate(std::path::Path::new("src/vfs/skel.rs"))?;
    Ok(())
}
