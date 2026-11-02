use tokio::io::AsyncReadExt;
use tokio::io::AsyncWriteExt;

mod eis;
mod xtest;

#[derive(clap::Parser, Debug)]
pub struct Args {
    /// Server address as host:port, such as arch.local:24801
    address: String,
}

pub async fn run(args: Args) -> crate::Return {
    let identity = crate::identity::load_or_generate()?;
    println!(
        "this machine: {}",
        crate::identity::fingerprint(&identity.certificate)
    );
    let verifier = std::sync::Arc::new(crate::identity::PeerVerifier::new(
        crate::identity::load_pin()?,
    ));
    let configuration = rustls::ClientConfig::builder_with_provider(crate::identity::provider())
        .with_protocol_versions(&[&rustls::version::TLS13])?
        .dangerous()
        .with_custom_certificate_verifier(verifier.clone())
        .with_client_auth_cert(vec![identity.certificate.clone()], identity.key)?;
    let connector = tokio_rustls::TlsConnector::from(std::sync::Arc::new(configuration));

    let mut loopback = tokio::process::Command::new("pw-loopback");
    loopback
        .args([
            "--name",
            "teleput-microphone",
            "--channels",
            "1",
            "--channel-map",
            "[ MONO ]",
            "--capture-props",
            "{ media.class = Audio/Sink node.name = teleput-microphone-sink }",
            "--playback-props",
            "{ media.class = Audio/Source node.name = teleput-microphone node.description = \"teleput microphone\" }",
        ])
        .kill_on_drop(true);
    // kill_on_drop never runs when teleput itself is killed by a signal, and pw-loopback holds no pipe to notice it.
    unsafe {
        loopback.pre_exec(|| {
            Ok(rustix::process::set_parent_process_death_signal(Some(
                rustix::process::Signal::TERM,
            ))?)
        });
    }
    let _loopback = loopback
        .spawn()
        .inspect_err(|e| eprintln!("microphone not provided: {e}"));

    let link = async move {
        let stream = tokio::net::TcpStream::connect(&args.address).await?;
        crate::protocol::configure(&stream)?;
        let stream = connector
            .connect(
                rustls::pki_types::ServerName::try_from(crate::protocol::SERVER_NAME)?,
                stream,
            )
            .await?;
        if !verifier.paired() {
            let peer = verifier
                .presented()
                .ok_or_else(|| error::error!("server presented no certificate"))?;
            crate::identity::confirm_pairing(&peer)?;
        }
        Ok::<Link, error::Error>(Link {
            address: args.address,
            stream,
            received: Vec::new(),
            player: None,
        })
    };
    if std::env::var_os("WAYLAND_DISPLAY").is_some() {
        eis::run(link).await
    } else {
        xtest::run(link).await
    }
}

struct Link {
    address: String,
    stream: tokio_rustls::client::TlsStream<tokio::net::TcpStream>,
    received: Vec<u8>,
    player: Option<tokio::sync::mpsc::Sender<Vec<u8>>>,
}

impl Link {
    async fn receive(&mut self) -> Result<Vec<crate::protocol::Message>, error::Error> {
        let mut chunk = [0u8; 4096];
        let read = self.stream.read(&mut chunk).await?;
        if read == 0 {
            error::bail!("connection closed by server");
        }
        self.received.extend_from_slice(&chunk[..read]);
        let mut messages = Vec::new();
        for message in crate::protocol::Message::parse(&mut self.received)? {
            match message {
                crate::protocol::Message::Hello { version } => {
                    if version != crate::protocol::VERSION {
                        error::bail!(
                            "server speaks version {version}, expected {}",
                            crate::protocol::VERSION
                        );
                    }
                    eprintln!("connected {}", self.address);
                }
                crate::protocol::Message::Audio { samples } => {
                    let player = self.player.get_or_insert_with(|| {
                        let (sender, mut receiver) = tokio::sync::mpsc::channel::<Vec<u8>>(2);
                        tokio::spawn(async move {
                            let played = async {
                                let mut child = tokio::process::Command::new("pw-cat")
                                    .arg("--playback")
                                    .args(crate::protocol::AUDIO_FORMAT)
                                    .args(["--target", "teleput-microphone-sink"])
                                    // Without these WirePlumber links a stream whose target is missing to the default sink: https://pipewire.pages.freedesktop.org/wireplumber/policies/linking.html
                                    .args(["-P", "{ node.dont-fallback = true node.linger = true node.dont-reconnect = true }"])
                                    .arg("-")
                                    .stdin(std::process::Stdio::piped())
                                    .kill_on_drop(true)
                                    .spawn()?;
                                let mut input = child.stdin.take().unwrap();
                                // pw-cat reads one graph cycle per process, so whatever the pipe holds stays as latency: https://github.com/PipeWire/pipewire/blob/master/src/tools/pw-cat.c
                                rustix::pipe::fcntl_setpipe_size(&input, 4096)?;
                                while let Some(samples) = receiver.recv().await {
                                    input.write_all(&samples).await?;
                                }
                                Ok::<(), error::Error>(())
                            };
                            if let Err(e) = played.await {
                                eprintln!("microphone not provided: {e}");
                            }
                        });
                        sender
                    });
                    let _ = player.try_send(samples.0);
                }
                message => messages.push(message),
            }
        }
        Ok(messages)
    }

    async fn announce(&mut self, screens: &[crate::protocol::Message]) -> Result<(), error::Error> {
        let mut buffer = Vec::new();
        crate::protocol::Message::Screens {
            count: u32::try_from(screens.len())?,
        }
        .encode(&mut buffer)?;
        for screen in screens {
            screen.encode(&mut buffer)?;
        }
        crate::protocol::transmit(&mut self.stream, &buffer).await
    }

    async fn send(&mut self, message: crate::protocol::Message) -> Result<(), error::Error> {
        let mut buffer = Vec::new();
        message.encode(&mut buffer)?;
        crate::protocol::transmit(&mut self.stream, &buffer).await
    }
}
