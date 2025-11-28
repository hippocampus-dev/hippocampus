Hippocampus – High‑Signal Development Notes (Project‑Specific)

Audience: Experienced Rust/Go/Python developers working in this monorepo. This document captures only repo‑specific, proven steps to build, test, and debug efficiently. Verified on 2025-09-05.

1) Build and Configuration (Rust workspace)
- Toolchain pin
  - Pinned nightly: nightly-2025-07-20 (rust-toolchain.toml). Ensure it is installed:
    - rustup toolchain install nightly-2025-07-20
    - rustup component add clippy rustfmt --toolchain nightly-2025-07-20
  - Edition: 2024 across crates (see Cargo.toml members).

- Makefile shortcuts (root)
  - make fmt → cargo fmt
  - make lint → cargo clippy --fix (serialized via hack/serial-makefile)
  - make tidy → cargo udeps --all-targets --all-features (requires nightly and cargo-udeps)
  - make test → cargo test (entire workspace)
  - make targets → cross build/test for x86_64-unknown-linux-gnu and musl via pinned images
  - make dev → watch build loop (watchexec + mold), also trusts ./.mitmproxy/mitmproxy-ca-cert.pem if present

- Cross compilation with cross
  - Install: cargo install cross
  - Targets pinned in Cross.toml:
    - ghcr.io/kaidotio/hippocampus/cross/x86_64-unknown-linux-gnu:main
    - ghcr.io/kaidotio/hippocampus/cross/x86_64-unknown-linux-musl:main
  - Example: cross build --release --target x86_64-unknown-linux-musl -p hippocampus-standalone
  - If GHCR pulls fail in corporate environments: docker login ghcr.io and ensure CA trust (see Dev watch below).

- Dev watch and corporate CA
  - make dev will trust a local MITM CA at ./.mitmproxy/mitmproxy-ca-cert.pem via system tools (update-ca-* family) and then run a watchexec loop.
  - Recommend installing mold or lld for much faster linking locally.

- Python/Go subprojects (when relevant to your task)
  - Python uses UV; lock is uv.lock. The Makefile has a uv target to freeze locks.
  - Go modules exist for some tools (armyknife, etc.); run make gomod to tidy.

2) Testing: configuration, running, and adding new tests
- Quick patterns
  - Whole workspace: cargo test
  - One crate: cargo test -p <crate>
  - One integration test file within a crate: cargo test -p <crate> --test <file_stem>
  - With logs: RUST_LOG=debug cargo test -p <crate> -- --nocapture

- Verified example (performed 2025-09-05)
  - Crate: packages/elapsed (lightweight utility crate)
  - Temporary test created at packages/elapsed/tests/temp_smoke.rs:
    ----------------------------------------------------------------
    use std::time::{Duration, Instant};
    use elapsed::SerializableTime;

    #[test]
    fn temp_smoke_serialization_roundtrip() {
        let t = SerializableTime::new(Instant::now());
        let s = serde_json::to_string(&t).expect("serialize");
        let de: SerializableTime = serde_json::from_str(&s).expect("deserialize");
        let e = de.elapsed();
        assert!(e < Duration::from_secs(5), "elapsed too large: {:?}", e);
    }
    ----------------------------------------------------------------
  - Command executed:
    cargo test -p elapsed --test temp_smoke
  - Observed result:
    1 passed; 0 failed; finished in ~0s (after first compile). Exact sample output:
      test temp_smoke_serialization_roundtrip ... ok
  - Cleanup: the temporary test file was removed after verification to avoid polluting the repo.

- Guidelines for tests in this repo
  - Prefer crate-local integration tests (tests/*.rs) for cross-module checks; unit tests can live next to code (mod tests in src/*).
  - Keep tests dependency-free and offline by default; avoid network and external services in examples.
  - For time-based checks (elapsed crate), allow small deltas; SerializableTime approximates Instant via SystemTime.
  - Scope runs narrowly (per crate or per test file) to retain fast feedback loops in this large workspace.
  - For platform-sensitive changes, validate with cross test before opening PRs:
    cross test -p <crate> --target x86_64-unknown-linux-musl

3) Additional Development Information
- Code style and static checks
  - Format with make fmt (cargo fmt). Edition 2024 features are allowed under pinned nightly.
  - Lint with make lint (cargo clippy --fix). Review diffs as fixes are applied automatically.
  - Detect unused deps: make tidy (cargo udeps). Requires nightly and cargo-udeps installed for the pinned toolchain.

- Debugging and logs
  - Enable backtraces: RUST_BACKTRACE=1 cargo test -p <crate>
  - Verbose logs: RUST_LOG=debug or module-specific log filters.
  - For very slow links locally, install mold or lld; make dev assumes mold if available.

- CI/CD and repo conventions
  - GitHub Actions are under .github/workflows/; expect formatting and clippy enforcement.
  - Some jobs use dynamic runners and Claude-based review.

- Known pitfalls
  - Nightly pin may require bumping if rustup channels rotate. Update rust-toolchain.toml consistently across the workspace.
  - cross requires Docker with network access; GHCR pulls may need docker login ghcr.io and trusted CA setup (see Dev watch).
  - cargo udeps requires nightly and the cargo-udeps binary installed in the pinned toolchain.

4) Minimal quickstart
- One-time setup:
  - rustup toolchain install nightly-2025-07-20
  - rustup component add clippy rustfmt --toolchain nightly-2025-07-20
  - cargo install cross watchexec-cli (optional but recommended); install mold via your package manager
- Daily loop:
  - make fmt && make lint
  - cargo test -p <crate> (or make test for the whole workspace)
  - cross test -p <crate> --target x86_64-unknown-linux-musl before landing platform-sensitive changes

Notes about this document
- All steps above are repo-specific and have been validated where marked “Verified”.
- Any temporary files created during the verification process (e.g., temp_smoke.rs) must be removed after use to keep the repo clean.
