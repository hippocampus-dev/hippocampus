use clap::Parser;

mod client;
mod fingerprint;
mod identity;
mod protocol;
#[cfg(target_os = "linux")]
mod server;
#[cfg(target_os = "linux")]
mod x11;

type Return = Result<(), error::Error>;

/// teleput forwards the keyboard and mouse of an X11 host to a Wayland, X11 or Windows host on the same LAN.
#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(subcommand)]
    sub_command: SubCommand,
}

#[derive(clap::Parser, Debug)]
enum SubCommand {
    /// Capture input on this X11 host and send it to the paired clients
    #[cfg(target_os = "linux")]
    Server(server::Args),
    /// Receive input from the paired server and inject it into this Wayland, X11 or Windows session
    Client(client::Args),
    /// Print the fingerprint of this machine's certificate, generating it if missing
    Fingerprint(fingerprint::Args),
}

#[tokio::main]
async fn main() -> Return {
    let args: Args = Args::parse();
    match args.sub_command {
        #[cfg(target_os = "linux")]
        SubCommand::Server(args) => server::run(args).await,
        SubCommand::Client(args) => client::run(args).await,
        SubCommand::Fingerprint(args) => fingerprint::run(args).await,
    }
}
