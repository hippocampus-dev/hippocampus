const DIRECTORY: &str = ".husky";

fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let output = std::process::Command::new("git")
        .args(["config", "core.hooksPath"])
        .output()?;
    if output.stdout.is_empty() {
        std::process::Command::new("git")
            .args(["config", "core.hooksPath", DIRECTORY])
            .spawn()?;
    }
    Ok(())
}
