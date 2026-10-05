use tokio::io::AsyncReadExt;
use x11rb::connection::Connection;
use x11rb::protocol::xfixes::ConnectionExt as _;
use x11rb::protocol::xinput::ConnectionExt as _;
use x11rb::protocol::xkb::ConnectionExt as _;
use x11rb::protocol::xproto::ConnectionExt as _;

// evdev KEY_SCROLLLOCK (70) plus the X11 keycode offset of 8.
const SCROLL_LOCK_KEYCODE: u8 = 78;

const EDGE_INSET: i16 = 8;

#[derive(clap::ValueEnum, Clone, Copy, Debug)]
pub enum Edge {
    Left,
    Right,
    Top,
    Bottom,
}

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(long, default_value_t = 24801)]
    port: u16,

    /// Screen edge the pointer crosses to move onto the client
    #[clap(long, value_enum, default_value = "right")]
    edge: Edge,
}

#[derive(Clone, Copy, Debug, PartialEq)]
struct Area {
    x: f32,
    y: f32,
    width: f32,
    height: f32,
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
    let mut configuration =
        rustls::ServerConfig::builder_with_provider(crate::identity::provider())
            .with_protocol_versions(&[&rustls::version::TLS13])?
            .with_client_cert_verifier(verifier.clone())
            .with_single_cert(vec![identity.certificate.clone()], identity.key)?;
    // A resumed TLS 1.3 handshake skips ClientCertVerifier (rustls src/server/tls13.rs).
    configuration.send_tls13_tickets = 0;
    let acceptor = tokio_rustls::TlsAcceptor::from(std::sync::Arc::new(configuration));
    let listener = tokio::net::TcpListener::bind(("::", args.port)).await?;

    let (sender, mut receiver) = tokio::sync::mpsc::unbounded_channel();
    // Some while a client is connected and has an absolute pointer to move.
    let screen = std::sync::Arc::new(std::sync::Mutex::new(None::<Area>));
    let mut capture = {
        let screen = screen.clone();
        let edge = args.edge;
        tokio::task::spawn_blocking(move || -> Result<(), error::Error> {
            let (connection, screen_number) = x11rb::connect(None)?;
            let root_screen = &connection.setup().roots[screen_number];
            let root = root_screen.root;
            let width = root_screen.width_in_pixels as i16;
            let height = root_screen.height_in_pixels as i16;

            connection.change_window_attributes(
                root,
                &x11rb::protocol::xproto::ChangeWindowAttributesAux::new()
                    .event_mask(x11rb::protocol::xproto::EventMask::STRUCTURE_NOTIFY),
            )?;
            connection.xinput_xi_query_version(2, 2)?.reply()?;
            connection.xfixes_query_version(4, 0)?.reply()?;
            connection.xkb_use_extension(1, 0)?.reply()?;
            // X11 autorepeat otherwise delivers a held key as repeated release/press pairs, on top of the client's own repeat.
            connection
                .xkb_per_client_flags(
                    x11rb::protocol::xkb::ID::USE_CORE_KBD.into(),
                    x11rb::protocol::xkb::PerClientFlag::DETECTABLE_AUTO_REPEAT,
                    x11rb::protocol::xkb::PerClientFlag::DETECTABLE_AUTO_REPEAT,
                    x11rb::protocol::xkb::BoolCtrl::from(0u32),
                    x11rb::protocol::xkb::BoolCtrl::from(0u32),
                    x11rb::protocol::xkb::BoolCtrl::from(0u32),
                )?
                .reply()?;
            // The master pointer's raw events are withheld from the client grabbing it on root (FilterRawEvents): https://gitlab.freedesktop.org/xorg/xserver/-/blob/master/dix/events.c
            connection.xinput_xi_select_events(
                root,
                &[x11rb::protocol::xinput::EventMask {
                    deviceid: x11rb::protocol::xinput::Device::ALL.into(),
                    mask: vec![x11rb::protocol::xinput::XIEventMask::RAW_MOTION],
                }],
            )?;
            connection
                .grab_key(
                    false,
                    root,
                    x11rb::protocol::xproto::ModMask::ANY,
                    SCROLL_LOCK_KEYCODE,
                    x11rb::protocol::xproto::GrabMode::ASYNC,
                    x11rb::protocol::xproto::GrabMode::ASYNC,
                )?
                .check()?;

            let mut capture = Capture {
                connection,
                root,
                width,
                height,
                edge,
                sender,
                active: false,
                pressed_keys: std::collections::HashSet::new(),
                area: Area {
                    x: 0.0,
                    y: 0.0,
                    width: 0.0,
                    height: 0.0,
                },
                position: (0.0, 0.0),
            };
            loop {
                let event = capture.connection.wait_for_event()?;
                let area = *screen.lock().unwrap();
                match area {
                    Some(area) => capture.area = area,
                    None if capture.active => capture.leave()?,
                    None => {}
                }
                match event {
                    x11rb::protocol::Event::ConfigureNotify(event) if event.window == root => {
                        capture.width = event.width as i16;
                        capture.height = event.height as i16;
                    }
                    x11rb::protocol::Event::XinputRawMotion(event)
                        if event.deviceid == event.sourceid =>
                    {
                        if capture.active {
                            let mut values = event.axisvalues.iter();
                            for axis in 0..event.valuator_mask.len() * 32 {
                                if event.valuator_mask[axis / 32] & (1 << (axis % 32)) == 0 {
                                    continue;
                                }
                                let Some(value) = values.next() else {
                                    break;
                                };
                                let delta = (value.integral as f64
                                    + value.frac as f64 / 4294967296.0)
                                    as f32;
                                match axis {
                                    0 => capture.position.0 += delta,
                                    1 => capture.position.1 += delta,
                                    _ => {}
                                }
                            }
                            let area = capture.area;
                            let (x, y) = capture.position;
                            let crossed = match edge {
                                Edge::Right => x < area.x,
                                Edge::Left => x > area.x + area.width - 1.0,
                                Edge::Top => y > area.y + area.height - 1.0,
                                Edge::Bottom => y < area.y,
                            };
                            if crossed {
                                capture.leave()?;
                            } else {
                                capture.position = (
                                    x.clamp(area.x, area.x + area.width - 1.0),
                                    y.clamp(area.y, area.y + area.height - 1.0),
                                );
                                capture.send(crate::protocol::Message::Motion {
                                    x: capture.position.0,
                                    y: capture.position.1,
                                })?;
                            }
                        } else if area.is_some() {
                            let pointer = capture.connection.query_pointer(root)?.reply()?;
                            let at_edge = match edge {
                                Edge::Left => pointer.root_x <= 0,
                                Edge::Right => pointer.root_x >= capture.width - 1,
                                Edge::Top => pointer.root_y <= 0,
                                Edge::Bottom => pointer.root_y >= capture.height - 1,
                            };
                            if at_edge {
                                capture.enter(pointer.root_x, pointer.root_y)?;
                            }
                        }
                    }
                    x11rb::protocol::Event::KeyPress(event)
                        if event.detail == SCROLL_LOCK_KEYCODE =>
                    {
                        if capture.active {
                            capture.leave()?;
                        } else if area.is_some() {
                            let pointer = capture.connection.query_pointer(root)?.reply()?;
                            capture.enter(pointer.root_x, pointer.root_y)?;
                        }
                    }
                    x11rb::protocol::Event::KeyPress(event) if capture.active => {
                        if capture.pressed_keys.insert(event.detail) {
                            capture.send(crate::protocol::Message::Key {
                                key: event.detail as u32 - 8,
                                pressed: true,
                            })?;
                        }
                    }
                    x11rb::protocol::Event::KeyRelease(event)
                        if capture.active && event.detail != SCROLL_LOCK_KEYCODE =>
                    {
                        if capture.pressed_keys.remove(&event.detail) {
                            capture.send(crate::protocol::Message::Key {
                                key: event.detail as u32 - 8,
                                pressed: false,
                            })?;
                        }
                    }
                    x11rb::protocol::Event::ButtonPress(event) if capture.active => {
                        // X11 reports the wheel as buttons 4-7.
                        // libei counts one notch as 120: https://libinput.pages.freedesktop.org/libei/interfaces/ei_scroll/index.html
                        let message = match event.detail {
                            4 => crate::protocol::Message::Scroll { dx: 0, dy: -120 },
                            5 => crate::protocol::Message::Scroll { dx: 0, dy: 120 },
                            6 => crate::protocol::Message::Scroll { dx: -120, dy: 0 },
                            7 => crate::protocol::Message::Scroll { dx: 120, dy: 0 },
                            detail => match button(detail) {
                                Some(button) => crate::protocol::Message::Button {
                                    button,
                                    pressed: true,
                                },
                                None => continue,
                            },
                        };
                        capture.send(message)?;
                    }
                    x11rb::protocol::Event::ButtonRelease(event) if capture.active => {
                        if let Some(button) = button(event.detail) {
                            capture.send(crate::protocol::Message::Button {
                                button,
                                pressed: false,
                            })?;
                        }
                    }
                    _ => {}
                }
            }
        })
    };

    loop {
        let (stream, address) = tokio::select! {
            result = &mut capture => return result?,
            accepted = listener.accept() => accepted?,
        };
        crate::protocol::configure(&stream)?;
        // A peer that never finishes the handshake would otherwise hold the only client slot.
        let mut stream =
            match tokio::time::timeout(std::time::Duration::from_secs(5), acceptor.accept(stream))
                .await
            {
                Ok(Ok(stream)) => stream,
                Ok(Err(e)) => {
                    eprintln!("rejected {address}: {e}");
                    continue;
                }
                Err(e) => {
                    eprintln!("rejected {address}: {e}");
                    continue;
                }
            };
        if !verifier.paired() {
            println!("pairing with {address}");
            let peer = verifier
                .presented()
                .ok_or_else(|| error::error!("client presented no certificate"))?;
            match crate::identity::confirm_pairing(&peer) {
                Ok(pin) => verifier.set_pin(pin),
                Err(e) => {
                    eprintln!("rejected {address}: {e}");
                    continue;
                }
            }
        }
        let mut buffer = Vec::new();
        crate::protocol::Message::Hello {
            version: crate::protocol::VERSION,
        }
        .encode(&mut buffer)?;
        if let Err(e) = crate::protocol::transmit(&mut stream, &buffer).await {
            eprintln!("lost {address}: {e}");
            continue;
        }
        while receiver.try_recv().is_ok() {}
        eprintln!("connected {address}");
        let mut received = Vec::new();
        let mut chunk = [0u8; 4096];
        let reason = loop {
            tokio::select! {
                result = &mut capture => return result?,
                message = receiver.recv() => {
                    let Some(message) = message else {
                        error::bail!("capture stopped sending");
                    };
                    buffer.clear();
                    message.encode(&mut buffer)?;
                    while let Ok(message) = receiver.try_recv() {
                        message.encode(&mut buffer)?;
                    }
                    if let Err(e) = crate::protocol::transmit(&mut stream, &buffer).await {
                        break e.to_string();
                    }
                }
                read = stream.read(&mut chunk) => {
                    let read = match read {
                        Ok(0) => break "closed by client".to_string(),
                        Ok(read) => read,
                        Err(e) => break e.to_string(),
                    };
                    received.extend_from_slice(&chunk[..read]);
                    let messages = match crate::protocol::Message::parse(&mut received) {
                        Ok(messages) => messages,
                        Err(e) => break e.to_string(),
                    };
                    for message in messages {
                        if let crate::protocol::Message::Screen { x, y, width, height } = message {
                            *screen.lock().unwrap() = (width > 0 && height > 0).then_some(Area {
                                x: x as f32,
                                y: y as f32,
                                width: width as f32,
                                height: height as f32,
                            });
                        }
                    }
                }
            }
        };
        *screen.lock().unwrap() = None;
        eprintln!("disconnected {address}: {reason}");
    }
}

struct Capture<C: Connection> {
    connection: C,
    root: x11rb::protocol::xproto::Window,
    width: i16,
    height: i16,
    edge: Edge,
    sender: tokio::sync::mpsc::UnboundedSender<crate::protocol::Message>,
    active: bool,
    pressed_keys: std::collections::HashSet<u8>,
    area: Area,
    position: (f32, f32),
}

impl<C: Connection> Capture<C> {
    fn send(&self, message: crate::protocol::Message) -> Result<(), error::Error> {
        self.sender
            .send(message)
            .map_err(|_| error::error!("server loop is gone"))
    }

    fn enter(&mut self, pointer_x: i16, pointer_y: i16) -> Result<(), error::Error> {
        let pointer = self
            .connection
            .grab_pointer(
                false,
                self.root,
                x11rb::protocol::xproto::EventMask::BUTTON_PRESS
                    | x11rb::protocol::xproto::EventMask::BUTTON_RELEASE,
                x11rb::protocol::xproto::GrabMode::ASYNC,
                x11rb::protocol::xproto::GrabMode::ASYNC,
                self.root,
                x11rb::NONE,
                x11rb::CURRENT_TIME,
            )?
            .reply()?;
        // Another client, such as an open menu, can already hold the grab.
        if pointer.status != x11rb::protocol::xproto::GrabStatus::SUCCESS {
            return Ok(());
        }
        let keyboard = self
            .connection
            .grab_keyboard(
                false,
                self.root,
                x11rb::CURRENT_TIME,
                x11rb::protocol::xproto::GrabMode::ASYNC,
                x11rb::protocol::xproto::GrabMode::ASYNC,
            )?
            .reply()?;
        if keyboard.status != x11rb::protocol::xproto::GrabStatus::SUCCESS {
            self.connection.ungrab_pointer(x11rb::CURRENT_TIME)?;
            self.connection.flush()?;
            return Ok(());
        }
        self.connection.xfixes_hide_cursor(self.root)?;
        self.connection.flush()?;
        self.active = true;
        let area = self.area;
        let x = area.x + pointer_x as f32 / self.width as f32 * area.width;
        let y = area.y + pointer_y as f32 / self.height as f32 * area.height;
        self.position = match self.edge {
            Edge::Right => (area.x, y),
            Edge::Left => (area.x + area.width - 1.0, y),
            Edge::Top => (x, area.y + area.height - 1.0),
            Edge::Bottom => (x, area.y),
        };
        self.send(crate::protocol::Message::Enter)?;
        self.send(crate::protocol::Message::Motion {
            x: self.position.0,
            y: self.position.1,
        })
    }

    fn leave(&mut self) -> Result<(), error::Error> {
        self.active = false;
        self.pressed_keys.clear();
        let area = self.area;
        let x = ((self.position.0 - area.x) / area.width * self.width as f32) as i16;
        let y = ((self.position.1 - area.y) / area.height * self.height as f32) as i16;
        let (x, y) = match self.edge {
            Edge::Left => (EDGE_INSET, y),
            Edge::Right => (self.width - 1 - EDGE_INSET, y),
            Edge::Top => (x, EDGE_INSET),
            Edge::Bottom => (x, self.height - 1 - EDGE_INSET),
        };
        self.connection.ungrab_keyboard(x11rb::CURRENT_TIME)?;
        self.connection.ungrab_pointer(x11rb::CURRENT_TIME)?;
        self.connection.xfixes_show_cursor(self.root)?;
        self.connection
            .warp_pointer(x11rb::NONE, self.root, 0, 0, 0, 0, x, y)?;
        self.connection.flush()?;
        self.send(crate::protocol::Message::Leave)
    }
}

// X11 button numbers to the Linux input-event codes libei takes (linux/input-event-codes.h).
fn button(detail: u8) -> Option<u32> {
    match detail {
        1 => Some(0x110),
        2 => Some(0x112),
        3 => Some(0x111),
        8 => Some(0x113),
        9 => Some(0x114),
        _ => None,
    }
}
