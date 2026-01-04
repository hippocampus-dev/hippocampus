fn main() -> Result<(), Box<dyn std::error::Error>> {
    let llama_cpp_path = std::path::PathBuf::from("llama.cpp");
    let src_path = std::path::PathBuf::from("src");

    if cfg!(feature = "cuda") {
        build_with_cmake(&llama_cpp_path, &src_path)?;
    } else {
        build_with_cc(&llama_cpp_path, &src_path)?;
    }

    println!("cargo:rerun-if-changed=src/wrapper.h");
    println!("cargo:rerun-if-changed=build.rs");

    Ok(())
}

fn build_with_cmake(
    llama_cpp_path: &std::path::Path,
    src_path: &std::path::Path,
) -> Result<(), Box<dyn std::error::Error>> {
    let llama_include = llama_cpp_path.join("include");
    let ggml_path = llama_cpp_path.join("ggml");
    let ggml_include = ggml_path.join("include");

    let d = cmake::Config::new(llama_cpp_path)
        .define("GGML_CUDA", "ON")
        .define("GGML_OPENMP", "ON")
        .define("CMAKE_BUILD_TYPE", "Release")
        .define("BUILD_SHARED_LIBS", "OFF")
        .define("LLAMA_BUILD_TESTS", "OFF")
        .define("LLAMA_BUILD_TOOLS", "OFF")
        .define("LLAMA_BUILD_EXAMPLES", "OFF")
        .define("LLAMA_BUILD_SERVER", "OFF")
        .build();

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

    let mut ggml_build = cc::Build::new();
    ggml_build
        .include(&ggml_include)
        .include(&ggml_src)
        .include(ggml_src.join("ggml-cpu"))
        .define("GGML_USE_CPU", None)
        .define("GGML_USE_OPENMP", None)
        .file(ggml_src.join("ggml.c"))
        .file(ggml_src.join("ggml-alloc.c"))
        .file(ggml_src.join("ggml-quants.c"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu.c"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu-quants.c"))
        .warnings(false);

    if cfg!(target_os = "linux") {
        ggml_build.define("_GNU_SOURCE", None);
    }

    ggml_build.flag("-fopenmp");
    ggml_build.compile("ggml");

    let mut ggml_cpp_build = cc::Build::new();
    ggml_cpp_build
        .cpp(true)
        .flag_if_supported("-std=c++17")
        .include(&ggml_include)
        .include(&ggml_src)
        .include(ggml_src.join("ggml-cpu"))
        .define("GGML_USE_CPU", None)
        .define("GGML_USE_OPENMP", None)
        .file(ggml_src.join("ggml-backend.cpp"))
        .file(ggml_src.join("ggml-backend-reg.cpp"))
        .file(ggml_src.join("ggml-threading.cpp"))
        .file(ggml_src.join("gguf.cpp"))
        .file(ggml_src.join("ggml-opt.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu-traits.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu-aarch64.cpp"))
        .file(ggml_src.join("ggml-cpu").join("ggml-cpu-hbm.cpp"))
        .file(ggml_src.join("ggml-cpu").join("cpu-feats-x86.cpp"))
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
        .warnings(false);

    if cfg!(target_os = "linux") {
        ggml_cpp_build.define("_GNU_SOURCE", None);
    }

    ggml_cpp_build.flag("-fopenmp");
    ggml_cpp_build.compile("ggml-cpp");

    let mut llama_build = cc::Build::new();
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
        .file(llama_src.join("llama-graph.cpp"))
        .file(llama_src.join("llama-cparams.cpp"))
        .file(llama_src.join("llama-adapter.cpp"))
        .file(llama_src.join("llama-chat.cpp"))
        .file(llama_src.join("llama-io.cpp"))
        .file(llama_src.join("llama-memory.cpp"))
        .file(llama_src.join("llama-model-saver.cpp"))
        .file(llama_src.join("llama-quant.cpp"))
        .file(llama_src.join("unicode.cpp"))
        .file(llama_src.join("unicode-data.cpp"))
        .warnings(false);

    if cfg!(target_os = "linux") {
        llama_build.define("_GNU_SOURCE", None);
    }

    llama_build.flag("-fopenmp");
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
