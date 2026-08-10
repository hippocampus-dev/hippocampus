fn main() -> Result<(), Box<dyn std::error::Error>> {
    let llama_cpp_path = std::path::PathBuf::from("llama.cpp");
    let src_path = std::path::PathBuf::from("src");

    if cfg!(feature = "cuda") {
        build_with_cmake(&llama_cpp_path, &src_path)?;
    } else {
        build_with_cc(&llama_cpp_path, &src_path)?;
    }

    println!("cargo:rerun-if-env-changed=CUDACXX");
    println!("cargo:rerun-if-changed=src/wrapper.h");
    println!("cargo:rerun-if-changed=src/lib.rs");
    println!("cargo:rerun-if-changed=build.rs");

    Ok(())
}

/// Whether the CUDA toolkit resolves `<cuda/iterator>`. Probed by preprocessing
/// a one-line translation unit so the toolkit's own include path answers,
/// rather than guessing at an install prefix.
fn cuda_iterator_header_available() -> bool {
    let out_directory =
        std::env::var("OUT_DIR").unwrap_or_else(|_| std::env::temp_dir().display().to_string());
    let probe = std::path::Path::new(&out_directory).join("cuda_iterator_probe.cu");
    if std::fs::write(&probe, "#include <cuda/iterator>\n").is_err() {
        return false;
    }

    // Captured rather than inherited: cargo reads this build script's stdout as
    // `cargo:` directives, and the preprocessed output is not one.
    std::process::Command::new(std::env::var("CUDACXX").unwrap_or_else(|_| "nvcc".to_string()))
        .arg("-E")
        .arg(&probe)
        .arg("-o")
        .arg("/dev/null")
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

fn build_with_cmake(
    llama_cpp_path: &std::path::Path,
    src_path: &std::path::Path,
) -> Result<(), Box<dyn std::error::Error>> {
    let llama_include = llama_cpp_path.join("include");
    let ggml_path = llama_cpp_path.join("ggml");
    let ggml_include = ggml_path.join("include");

    let mut configuration = cmake::Config::new(llama_cpp_path);
    configuration
        .define("GGML_CUDA", "ON")
        .define("GGML_OPENMP", "ON")
        .define("CMAKE_BUILD_TYPE", "Release")
        .define("BUILD_SHARED_LIBS", "OFF")
        .define("LLAMA_BUILD_TESTS", "OFF")
        .define("LLAMA_BUILD_TOOLS", "OFF")
        .define("LLAMA_BUILD_EXAMPLES", "OFF")
        .define("LLAMA_BUILD_SERVER", "OFF");

    // ggml-cuda reaches for cuda::make_strided_iterator once CCCL reports
    // >= 3.1, but only includes <cub/cub.cuh>; from CCCL 3.4 (bundled with
    // CUDA 13.3) those factories live in <cuda/iterator>, so argsort.cu and
    // top-k.cu fail to compile. Force the header in until the vendored
    // llama.cpp carries the include itself -- but only where the toolkit has
    // it, because on CCCL 2.x the header is absent and ggml's own fallback
    // path already compiles, so forcing it there would break every
    // translation unit rather than fix two.
    if cuda_iterator_header_available() {
        configuration.define("CMAKE_CUDA_FLAGS", "-include cuda/iterator");
    }

    let d = configuration.build();

    println!("cargo:rustc-link-search=native={}/lib", d.display());
    println!("cargo:rustc-link-search=native={}/lib64", d.display());
    println!("cargo:rustc-link-search=native={}/build", d.display());
    println!("cargo:rustc-link-search=native={}/build/src", d.display());
    println!(
        "cargo:rustc-link-search=native={}/build/ggml/src",
        d.display()
    );

    println!("cargo:rustc-link-lib=static=ggml");
    println!("cargo:rustc-link-lib=static=ggml-base");
    println!("cargo:rustc-link-lib=static=ggml-cpu");
    println!("cargo:rustc-link-lib=static=ggml-cuda");
    println!("cargo:rustc-link-lib=static=llama");

    println!("cargo:rustc-link-search=native=/opt/cuda/lib64");
    println!("cargo:rustc-link-search=native=/opt/cuda/lib64/stubs");
    println!("cargo:rustc-link-search=native=/usr/local/cuda/lib64");
    println!("cargo:rustc-link-search=native=/usr/local/cuda/lib64/stubs");
    println!("cargo:rustc-link-search=native=/opt/cuda/targets/x86_64-linux/lib");
    println!("cargo:rustc-link-search=native=/opt/cuda/targets/x86_64-linux/lib/stubs");

    println!("cargo:rustc-link-lib=dylib=cuda");
    println!("cargo:rustc-link-lib=dylib=cudart");
    println!("cargo:rustc-link-lib=dylib=cublas");
    println!("cargo:rustc-link-lib=dylib=cublasLt");

    let mut builder =
        autocxx_build::Builder::new("src/lib.rs", [src_path, &llama_include, &ggml_include])
            .build()?;

    builder
        .flag_if_supported("-std=c++17")
        .include(&llama_include)
        .include(&ggml_include)
        .define("GGML_USE_OPENMP", None)
        .define("GGML_USE_CUDA", None)
        .flag("-fopenmp")
        .compile("llama_cpp_bridge");

    println!("cargo:rustc-link-lib=gomp");

    Ok(())
}

fn build_with_cc(
    llama_cpp_path: &std::path::Path,
    src_path: &std::path::Path,
) -> Result<(), Box<dyn std::error::Error>> {
    let ggml_path = llama_cpp_path.join("ggml");
    let llama_include = llama_cpp_path.join("include");
    let ggml_include = ggml_path.join("include");
    let llama_src = llama_cpp_path.join("src");
    let ggml_src = ggml_path.join("src");

    println!("cargo:rerun-if-env-changed=RUST_DE_LLAMA_MARCH");
    let march = std::env::var("RUST_DE_LLAMA_MARCH").unwrap_or_else(|_| "x86-64-v3".to_string());

    // The vendored C/C++ always compiles with NDEBUG, matching upstream
    // llama.cpp Release builds and the cmake/CUDA path above (which sets
    // CMAKE_BUILD_TYPE=Release). It is third-party code this repository does
    // not debug, and one of its plain `assert()`s is wrong: `wavtokenizer-dec`
    // trips a `build_inp_embd` shape assertion that upstream only ever compiles
    // out, which made `make dev` unable to serve audio at all. Cargo's own
    // profile still governs the Rust half.
    //
    // GGML_ASSERT carries the invariants that matter and is not NDEBUG-gated
    // (`ggml/include/ggml.h`: `#define GGML_ASSERT(x) if (!(x)) GGML_ABORT(..)`),
    // so it still aborts in every profile. The rest of what NDEBUG turns off is
    // upstream's own Release trade, taken here for the same reasons they take
    // it: the ~335 plain `assert()`s, the GGML_ABORT on memory-pool exhaustion
    // (`ggml/src/ggml.c`, which then returns NULL instead), GGML_UNREACHABLE's
    // diagnostic (it becomes `__builtin_unreachable()`), and the backend-registry
    // load diagnostics.
    let mut ggml_build = cc::Build::new();
    ggml_build.define("NDEBUG", None);
    ggml_build
        .include(&ggml_include)
        .include(&ggml_src)
        .include(ggml_src.join("ggml-cpu"))
        .define("GGML_USE_CPU", None)
        .define("GGML_USE_OPENMP", None)
        .define("GGML_VERSION", Some("\"0.9.5\""))
        .define("GGML_COMMIT", Some("\"unknown\""))
        .file(ggml_src.join("ggml.c"))
        .file(ggml_src.join("ggml-alloc.c"))
        .file(ggml_src.join("ggml-quants.c"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu.c"))
        .file(ggml_src.join("ggml-cpu").join("quants.c"))
        .file(
            ggml_src
                .join("ggml-cpu")
                .join("arch")
                .join("x86")
                .join("quants.c"),
        )
        .warnings(false);

    if cfg!(target_os = "linux") {
        ggml_build.define("_GNU_SOURCE", None);
    }

    ggml_build.flag("-fopenmp");
    ggml_build.flag(format!("-march={march}"));
    ggml_build.compile("ggml");

    let mut ggml_cpp_build = cc::Build::new();
    ggml_cpp_build.define("NDEBUG", None);
    ggml_cpp_build
        .cpp(true)
        .flag_if_supported("-std=c++17")
        .include(&ggml_include)
        .include(&ggml_src)
        .include(ggml_src.join("ggml-cpu"))
        .define("GGML_USE_CPU", None)
        .define("GGML_USE_OPENMP", None)
        .file(ggml_src.join("ggml-backend.cpp"))
        .file(ggml_src.join("ggml-backend-dl.cpp"))
        .file(ggml_src.join("ggml-backend-reg.cpp"))
        .file(ggml_src.join("ggml.cpp"))
        .file(ggml_src.join("ggml-threading.cpp"))
        .file(ggml_src.join("gguf.cpp"))
        .file(ggml_src.join("ggml-opt.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu.cpp"))
        .file(ggml_src.join("ggml-cpu").join("traits.cpp"))
        .file(ggml_src.join("ggml-cpu").join("hbm.cpp"))
        .file(ggml_src.join("ggml-cpu").join("repack.cpp"))
        .file(ggml_src.join("ggml-cpu").join("binary-ops.cpp"))
        .file(ggml_src.join("ggml-cpu").join("unary-ops.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ops.cpp"))
        .file(ggml_src.join("ggml-cpu").join("vec.cpp"))
        .file(
            ggml_src
                .join("ggml-cpu")
                .join("llamafile")
                .join("sgemm.cpp"),
        )
        .file(
            ggml_src
                .join("ggml-cpu")
                .join("arch")
                .join("x86")
                .join("repack.cpp"),
        )
        .file(
            ggml_src
                .join("ggml-cpu")
                .join("arch")
                .join("x86")
                .join("cpu-feats.cpp"),
        )
        .file(ggml_src.join("ggml-cpu").join("amx").join("amx.cpp"))
        .file(ggml_src.join("ggml-cpu").join("amx").join("mmq.cpp"))
        .warnings(false);

    if cfg!(target_os = "linux") {
        ggml_cpp_build.define("_GNU_SOURCE", None);
    }

    ggml_cpp_build.flag("-fopenmp");
    ggml_cpp_build.flag(format!("-march={march}"));
    ggml_cpp_build.compile("ggml-cpp");

    let mut llama_build = cc::Build::new();
    llama_build.define("NDEBUG", None);
    llama_build
        .cpp(true)
        .flag_if_supported("-std=c++17")
        .include(&llama_include)
        .include(&llama_src)
        .include(&ggml_include)
        .define("GGML_USE_CPU", None)
        .define("GGML_USE_OPENMP", None)
        .file(llama_src.join("llama.cpp"))
        .file(llama_src.join("llama-mmap.cpp"))
        .file(llama_src.join("llama-impl.cpp"))
        .file(llama_src.join("llama-model.cpp"))
        .file(llama_src.join("llama-model-loader.cpp"))
        .file(llama_src.join("llama-vocab.cpp"))
        .file(llama_src.join("llama-hparams.cpp"))
        .file(llama_src.join("llama-arch.cpp"))
        .file(llama_src.join("llama-batch.cpp"))
        .file(llama_src.join("llama-context.cpp"))
        .file(llama_src.join("llama-sampling.cpp"))
        .file(llama_src.join("llama-grammar.cpp"))
        .file(llama_src.join("llama-kv-cache.cpp"))
        .file(llama_src.join("llama-kv-cache-iswa.cpp"))
        .file(llama_src.join("llama-memory-hybrid.cpp"))
        .file(llama_src.join("llama-memory-hybrid-iswa.cpp"))
        .file(llama_src.join("llama-memory-recurrent.cpp"))
        .file(llama_src.join("llama-graph.cpp"))
        .file(llama_src.join("llama-cparams.cpp"))
        .file(llama_src.join("llama-adapter.cpp"))
        .file(llama_src.join("llama-chat.cpp"))
        .file(llama_src.join("llama-io.cpp"))
        .file(llama_src.join("llama-memory.cpp"))
        .file(llama_src.join("llama-model-saver.cpp"))
        .file(llama_src.join("llama-quant.cpp"))
        .file(llama_src.join("unicode.cpp"))
        .file(llama_src.join("unicode-data.cpp"));

    for entry in std::fs::read_dir(llama_src.join("models"))? {
        let path = entry?.path();
        if path.extension().and_then(|e| e.to_str()) == Some("cpp") {
            llama_build.file(&path);
        }
    }

    llama_build.warnings(false);

    if cfg!(target_os = "linux") {
        llama_build.define("_GNU_SOURCE", None);
    }

    llama_build.flag("-fopenmp");
    llama_build.flag(format!("-march={march}"));
    llama_build.compile("llama");

    let mut builder =
        autocxx_build::Builder::new("src/lib.rs", [src_path, &llama_include, &ggml_include])
            .build()?;

    builder
        .flag_if_supported("-std=c++17")
        .include(&llama_include)
        .include(&ggml_include)
        .define("GGML_USE_OPENMP", None)
        .flag("-fopenmp")
        .compile("llama_cpp_bridge");

    println!("cargo:rustc-link-lib=gomp");

    Ok(())
}
