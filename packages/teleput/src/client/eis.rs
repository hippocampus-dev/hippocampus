use futures_util::StreamExt;

const TEXT_MIME_TYPE: &str = "text/plain;charset=utf-8";

pub async fn run(
    link: impl std::future::Future<Output = Result<super::Link, error::Error>>,
) -> crate::Return {
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
    let clipboard = ashpd::desktop::clipboard::Clipboard::new().await?;
    let mut owner_changes = std::pin::pin!(
        clipboard
            .receive_selection_owner_changed::<ashpd::desktop::remote_desktop::RemoteDesktop>()
            .await?
    );
    let mut transfers = std::pin::pin!(
        clipboard
            .receive_selection_transfer::<ashpd::desktop::remote_desktop::RemoteDesktop>()
            .await?
    );
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
    // RequestClipboard is accepted only before Start: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Clipboard.html
    clipboard
        .request(
            &session,
            ashpd::desktop::clipboard::RequestClipboardOptions::default(),
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
    let mut offered = String::new();
    let (read_sender, mut reads) = tokio::sync::mpsc::unbounded_channel();
    let (write_sender, mut writes) = tokio::sync::mpsc::unbounded_channel();

    let mut link = link.await?;
    loop {
        tokio::select! {
            messages = link.receive() => {
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
                        crate::protocol::Message::Clipboard { text } => {
                            offered = text;
                            clipboard
                                .set_selection(
                                    &session,
                                    ashpd::desktop::clipboard::SetSelectionOptions::default()
                                        .set_mime_types(&[TEXT_MIME_TYPE]),
                                )
                                .await?;
                        }
                        _ => {}
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
                let screens: Vec<_> = injector
                    .devices
                    .iter()
                    .find(|device| device.has_capability(reis::event::DeviceCapability::PointerAbsolute))
                    .map(|device| {
                        device
                            .regions()
                            .iter()
                            .map(|region| crate::protocol::Message::Screen {
                                x: region.x,
                                y: region.y,
                                width: region.width,
                                height: region.height,
                            })
                            .collect()
                    })
                    .unwrap_or_default();
                if announced.as_ref() != Some(&screens) {
                    link.announce(&screens).await?;
                    announced = Some(screens);
                }
            }
            change = owner_changes.next() => {
                let Some((_, change)) = change else {
                    error::bail!("clipboard portal closed");
                };
                if change.session_is_owner() != Some(true)
                    && change.mime_types().iter().any(|mime_type| mime_type == TEXT_MIME_TYPE)
                {
                    match clipboard.selection_read(&session, TEXT_MIME_TYPE).await {
                        Ok(fd) => {
                            let read_sender = read_sender.clone();
                            tokio::task::spawn_blocking(move || {
                                let mut text = String::new();
                                let read = std::io::Read::read_to_string(&mut std::fs::File::from(std::os::fd::OwnedFd::from(fd)), &mut text);
                                let _ = read_sender.send(read.map(|_| text));
                            });
                        }
                        Err(e) => eprintln!("clipboard not read: {e}"),
                    }
                }
            }
            Some(read) = reads.recv() => {
                match read {
                    Ok(text) => link.send(crate::protocol::Message::Clipboard { text }).await?,
                    Err(e) => eprintln!("clipboard not read: {e}"),
                }
            }
            transfer = transfers.next() => {
                let Some((_, _, serial)) = transfer else {
                    error::bail!("clipboard portal closed");
                };
                let fd = clipboard.selection_write(&session, serial).await?;
                let text = offered.clone();
                let write_sender = write_sender.clone();
                tokio::task::spawn_blocking(move || {
                    let written = std::io::Write::write_all(&mut std::fs::File::from(std::os::fd::OwnedFd::from(fd)), text.as_bytes());
                    let _ = write_sender.send((serial, written.is_ok()));
                });
            }
            Some((serial, success)) = writes.recv() => {
                clipboard.selection_write_done(&session, serial, success).await?;
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
