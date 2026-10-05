use futures_util::StreamExt;
use tokio::io::AsyncReadExt;

#[derive(clap::Parser, Debug)]
pub struct Args {
    /// Server address as host:port, such as arch.local:24801
    address: String,
}

pub async fn run(args: Args) -> crate::Return {
    let address = args.address;
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

    // The portal may wait on a permission dialog, during which a connected server would already be capturing input.
    let token_path =
        crate::identity::base_directory("XDG_STATE_HOME", ".local/state")?.join("restore-token");
    let token = match std::fs::read_to_string(&token_path) {
        Ok(token) => Some(token.trim().to_string()),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => None,
        Err(e) => return Err(e.into()),
    };
    let remote_desktop = ashpd::desktop::remote_desktop::RemoteDesktop::new().await?;
    let session = remote_desktop
        .create_session(ashpd::desktop::CreateSessionOptions::default())
        .await?;
    remote_desktop
        .select_devices(
            &session,
            ashpd::desktop::remote_desktop::SelectDevicesOptions::default()
                .set_devices(
                    ashpd::desktop::remote_desktop::DeviceType::Keyboard
                        | ashpd::desktop::remote_desktop::DeviceType::Pointer,
                )
                .set_persist_mode(ashpd::desktop::PersistMode::ExplicitlyRevoked)
                .set_restore_token(token.as_deref()),
        )
        .await?;
    let selected = remote_desktop
        .start(
            &session,
            None,
            ashpd::desktop::remote_desktop::StartOptions::default(),
        )
        .await?
        .response()?;
    // A restore token is single-use: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html
    if let Some(token) = selected.restore_token() {
        crate::identity::write_private(&token_path, token.as_bytes())?;
    }
    let fd = remote_desktop
        .connect_to_eis(
            &session,
            ashpd::desktop::remote_desktop::ConnectToEISOptions::default(),
        )
        .await?;
    let context = reis::ei::Context::new(std::os::unix::net::UnixStream::from(fd))?;
    let (connection, mut events) = context
        .handshake_tokio(
            crate::protocol::SERVER_NAME,
            reis::ei::handshake::ContextType::Sender,
        )
        .await?;
    let mut injector = Injector {
        context,
        connection,
        devices: Vec::new(),
        pressed_keys: std::collections::HashSet::new(),
        pressed_buttons: std::collections::HashSet::new(),
    };
    let mut sequence = 0;
    let mut announced = None;

    let stream = tokio::net::TcpStream::connect(&address).await?;
    crate::protocol::configure(&stream)?;
    let mut stream = connector
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
    let mut received = Vec::new();
    let mut chunk = [0u8; 4096];
    let mut buffer = Vec::new();
    loop {
        tokio::select! {
            read = stream.read(&mut chunk) => {
                let messages = match read {
                    Ok(0) => Err(error::error!("connection closed by server")),
                    Ok(read) => {
                        received.extend_from_slice(&chunk[..read]);
                        crate::protocol::Message::parse(&mut received)
                    }
                    Err(e) => Err(e.into()),
                };
                let messages = match messages {
                    Ok(messages) => messages,
                    Err(e) => {
                        injector.release();
                        injector.context.flush()?;
                        return Err(e);
                    }
                };
                for message in messages {
                    match message {
                        crate::protocol::Message::Hello { version } => {
                            if version != crate::protocol::VERSION {
                                error::bail!(
                                    "server speaks version {version}, expected {}",
                                    crate::protocol::VERSION
                                );
                            }
                            eprintln!("connected {address}");
                        }
                        crate::protocol::Message::Enter | crate::protocol::Message::Screen { .. } => {}
                        crate::protocol::Message::Leave => injector.release(),
                        crate::protocol::Message::Motion { x, y } => injector.emit(
                            |pointer: &reis::ei::PointerAbsolute| pointer.motion_absolute(x, y),
                        ),
                        crate::protocol::Message::Scroll { dx, dy } => {
                            injector.emit(|scroll: &reis::ei::Scroll| scroll.scroll_discrete(dx, dy))
                        }
                        crate::protocol::Message::Button { button, pressed } => {
                            let state = if pressed {
                                injector.pressed_buttons.insert(button);
                                reis::ei::button::ButtonState::Press
                            } else {
                                injector.pressed_buttons.remove(&button);
                                reis::ei::button::ButtonState::Released
                            };
                            injector.emit(|device: &reis::ei::Button| device.button(button, state))
                        }
                        crate::protocol::Message::Key { key, pressed } => {
                            let state = if pressed {
                                injector.pressed_keys.insert(key);
                                reis::ei::keyboard::KeyState::Press
                            } else {
                                injector.pressed_keys.remove(&key);
                                reis::ei::keyboard::KeyState::Released
                            };
                            injector.emit(|keyboard: &reis::ei::Keyboard| keyboard.key(key, state))
                        }
                    }
                }
                injector.context.flush()?;
            }
            event = events.next() => {
                let Some(event) = event else {
                    error::bail!("EIS connection closed");
                };
                match event? {
                    reis::event::EiEvent::SeatAdded(event) => {
                        event.seat.bind_capabilities(
                            reis::event::DeviceCapability::PointerAbsolute
                                | reis::event::DeviceCapability::Button
                                | reis::event::DeviceCapability::Scroll
                                | reis::event::DeviceCapability::Keyboard,
                        );
                    }
                    reis::event::EiEvent::DeviceResumed(event) => {
                        event.device.device().start_emulating(event.serial, sequence);
                        sequence += 1;
                        injector.devices.push(event.device);
                    }
                    // Pausing releases the device's keys and buttons on the EIS side: https://libinput.pages.freedesktop.org/libei/interfaces/ei_device/index.html
                    reis::event::EiEvent::DevicePaused(reis::event::DevicePaused { device, .. })
                    | reis::event::EiEvent::DeviceRemoved(reis::event::DeviceRemoved { device }) => {
                        injector.devices.retain(|resumed| *resumed != device);
                        injector.pressed_keys.clear();
                        injector.pressed_buttons.clear();
                    }
                    reis::event::EiEvent::Disconnected(event) => {
                        error::bail!("EIS disconnected: {:?}", event.reason);
                    }
                    _ => {}
                }
                injector.context.flush()?;
                // A position outside the device's regions is silently discarded, which a box spanning several regions would produce in the gaps: https://libinput.pages.freedesktop.org/libei/interfaces/ei_pointer_absolute/index.html
                let region = injector
                    .devices
                    .iter()
                    .find(|device| device.has_capability(reis::event::DeviceCapability::PointerAbsolute))
                    .and_then(|device| device.regions().first());
                let screen = match region {
                    Some(region) => crate::protocol::Message::Screen {
                        x: region.x,
                        y: region.y,
                        width: region.width,
                        height: region.height,
                    },
                    None => crate::protocol::Message::Screen {
                        x: 0,
                        y: 0,
                        width: 0,
                        height: 0,
                    },
                };
                if announced != Some(screen) {
                    buffer.clear();
                    screen.encode(&mut buffer)?;
                    crate::protocol::transmit(&mut stream, &buffer).await?;
                    announced = Some(screen);
                }
            }
        }
    }
}

struct Injector {
    context: reis::ei::Context,
    connection: reis::event::Connection,
    devices: Vec<reis::event::Device>,
    pressed_keys: std::collections::HashSet<u32>,
    pressed_buttons: std::collections::HashSet<u32>,
}

impl Injector {
    fn emit<T: reis::ei::Interface>(&self, request: impl FnOnce(&T)) {
        let Some((device, interface)) = self
            .devices
            .iter()
            .find_map(|device| device.interface::<T>().map(|interface| (device, interface)))
        else {
            return;
        };
        request(&interface);
        let time = rustix::time::clock_gettime(rustix::time::ClockId::Monotonic);
        device.device().frame(
            self.connection.serial(),
            time.tv_sec as u64 * 1_000_000 + time.tv_nsec as u64 / 1_000,
        );
    }

    fn release(&mut self) {
        for key in std::mem::take(&mut self.pressed_keys) {
            self.emit(|keyboard: &reis::ei::Keyboard| {
                keyboard.key(key, reis::ei::keyboard::KeyState::Released)
            });
        }
        for button in std::mem::take(&mut self.pressed_buttons) {
            self.emit(|device: &reis::ei::Button| {
                device.button(button, reis::ei::button::ButtonState::Released)
            });
        }
    }
}
