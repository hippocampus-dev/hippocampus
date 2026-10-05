use clap::Parser;

mod client;
mod identity;
mod protocol;
mod server;

type Return = Result<(), error::Error>;

/// teleput forwards the keyboard and mouse of an X11 host to a Wayland host on the same LAN.
#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(subcommand)]
    sub_command: SubCommand,
}

#[derive(clap::Parser, Debug)]
enum SubCommand {
    /// Capture input on this X11 host and send it to the paired client
    Server(server::Args),
    /// Receive input from the paired server and inject it into this Wayland session
    Client(client::Args),
}

#[tokio::main]
async fn main() -> Return {
    let args: Args = Args::parse();
    match args.sub_command {
        SubCommand::Server(args) => server::run(args).await,
        SubCommand::Client(args) => client::run(args).await,
    }
}
