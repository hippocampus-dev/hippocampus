#[derive(clap::Parser, Debug)]
pub struct Args {}

pub async fn run(_args: Args) -> crate::Return {
    let identity = crate::identity::load_or_generate()?;
    println!("{}", crate::identity::fingerprint(&identity.certificate));
    Ok(())
}
