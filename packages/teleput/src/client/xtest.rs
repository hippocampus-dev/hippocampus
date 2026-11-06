use std::os::fd::AsRawFd;
use x11rb::connection::Connection;
use x11rb::protocol::xfixes::ConnectionExt as _;
use x11rb::protocol::xproto::ConnectionExt as _;
use x11rb::protocol::xtest::ConnectionExt as _;

pub async fn run(
    link: impl std::future::Future<Output = Result<super::Link, error::Error>>,
) -> crate::Return {
    let (connection, screen_number) = x11rb::connect(None)?;
    let root_screen = &connection.setup().roots[screen_number];
    let root = root_screen.root;
    let mut size = (root_screen.width_in_pixels, root_screen.height_in_pixels);
    let atoms = crate::x11::Atoms::new(&connection)?.reply()?;
    let window = connection.generate_id()?;
    connection.create_window(
        x11rb::COPY_DEPTH_FROM_PARENT,
        window,
        root,
        0,
        0,
        1,
        1,
        0,
        x11rb::protocol::xproto::WindowClass::INPUT_ONLY,
        x11rb::COPY_FROM_PARENT,
        &x11rb::protocol::xproto::CreateWindowAux::new(),
    )?;
    connection.change_window_attributes(
        root,
        &x11rb::protocol::xproto::ChangeWindowAttributesAux::new()
            .event_mask(x11rb::protocol::xproto::EventMask::STRUCTURE_NOTIFY),
    )?;
    connection.xtest_get_version(2, 2)?.reply()?;
    connection.xfixes_query_version(4, 0)?.reply()?;
    connection.xfixes_select_selection_input(
        window,
        atoms.CLIPBOARD,
        x11rb::protocol::xfixes::SelectionEventMask::SET_SELECTION_OWNER,
    )?;
    connection.flush()?;
    // A spawn_blocking thread parked in wait_for_event would keep the runtime from shutting down (tokio src/runtime/blocking/pool.rs).
    let readiness = tokio::io::unix::AsyncFd::with_interest(
        connection.stream().as_raw_fd(),
        tokio::io::Interest::READABLE,
    )?;
    let mut injector = Injector {
        connection: &connection,
        root,
        pressed_keys: std::collections::HashSet::new(),
        pressed_buttons: std::collections::HashSet::new(),
    };
    let mut offered = String::new();

    let mut link = link.await?;
    // The X server keeps keys pressed through XTest down after teleput exits (xserver Xext/xtest.c).
    let mut interrupt = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::interrupt())?;
    let mut terminate = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    let mut hangup = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::hangup())?;
    link.announce(&[crate::protocol::Message::Screen {
        x: 0,
        y: 0,
        width: size.0.into(),
        height: size.1.into(),
    }])
    .await?;
    loop {
        while let Some(event) = connection.poll_for_event()? {
            match event {
                x11rb::protocol::Event::ConfigureNotify(event)
                    if event.window == root && (event.width, event.height) != size =>
                {
                    size = (event.width, event.height);
                    link.announce(&[crate::protocol::Message::Screen {
                        x: 0,
                        y: 0,
                        width: size.0.into(),
                        height: size.1.into(),
                    }])
                    .await?;
                }
                x11rb::protocol::Event::XfixesSelectionNotify(event) if event.owner != window => {
                    connection.convert_selection(
                        window,
                        atoms.CLIPBOARD,
                        atoms.UTF8_STRING,
                        atoms.TELEPUT,
                        event.selection_timestamp,
                    )?;
                }
                x11rb::protocol::Event::SelectionNotify(event) if event.property != x11rb::NONE => {
                    if let Some(text) = crate::x11::read(&connection, window, &atoms, &event)? {
                        link.send(crate::protocol::Message::Clipboard { text })
                            .await?;
                    }
                }
                x11rb::protocol::Event::SelectionRequest(event) => {
                    crate::x11::answer(&connection, window, &atoms, &offered, &event)?;
                }
                _ => {}
            }
        }
        connection.flush()?;
        tokio::select! {
            ready = readiness.readable() => ready?.clear_ready(),
            _ = interrupt.recv() => return Ok(()),
            _ = terminate.recv() => return Ok(()),
            _ = hangup.recv() => return Ok(()),
            messages = link.receive() => {
                for message in messages? {
                    match message {
                        crate::protocol::Message::Leave => injector.release()?,
                        crate::protocol::Message::Motion { x, y } => injector.fake(
                            x11rb::protocol::xproto::MOTION_NOTIFY_EVENT,
                            0,
                            x as i16,
                            y as i16,
                        )?,
                        crate::protocol::Message::Scroll { dx, dy } => {
                            // X11 takes the wheel as buttons 4-7.
                            // libei counts one notch as 120: https://libinput.pages.freedesktop.org/libei/interfaces/ei_scroll/index.html
                            for (delta, backward, forward) in [(dy, 4, 5), (dx, 6, 7)] {
                                let detail = if delta < 0 { backward } else { forward };
                                for _ in 0..delta.unsigned_abs() / 120 {
                                    injector.fake(x11rb::protocol::xproto::BUTTON_PRESS_EVENT, detail, 0, 0)?;
                                    injector.fake(x11rb::protocol::xproto::BUTTON_RELEASE_EVENT, detail, 0, 0)?;
                                }
                            }
                        }
                        crate::protocol::Message::Button { button, pressed } => {
                            let Some(&(detail, _)) =
                                crate::x11::BUTTONS.iter().find(|(_, code)| *code == button)
                            else {
                                continue;
                            };
                            let type_ = if pressed {
                                injector.pressed_buttons.insert(detail);
                                x11rb::protocol::xproto::BUTTON_PRESS_EVENT
                            } else {
                                injector.pressed_buttons.remove(&detail);
                                x11rb::protocol::xproto::BUTTON_RELEASE_EVENT
                            };
                            injector.fake(type_, detail, 0, 0)?;
                        }
                        crate::protocol::Message::Key { key, pressed } => {
                            // evdev keycodes plus the X11 keycode offset of 8.
                            let detail = u8::try_from(key + 8)?;
                            let type_ = if pressed {
                                injector.pressed_keys.insert(detail);
                                x11rb::protocol::xproto::KEY_PRESS_EVENT
                            } else {
                                injector.pressed_keys.remove(&detail);
                                x11rb::protocol::xproto::KEY_RELEASE_EVENT
                            };
                            injector.fake(type_, detail, 0, 0)?;
                        }
                        crate::protocol::Message::Clipboard { text } => {
                            offered = text;
                            connection.set_selection_owner(
                                window,
                                atoms.CLIPBOARD,
                                x11rb::CURRENT_TIME,
                            )?;
                        }
                        _ => {}
                    }
                }
                connection.flush()?;
            }
        }
    }
}

struct Injector<'a, C: Connection> {
    connection: &'a C,
    root: x11rb::protocol::xproto::Window,
    pressed_keys: std::collections::HashSet<u8>,
    pressed_buttons: std::collections::HashSet<u8>,
}

impl<C: Connection> Injector<'_, C> {
    fn fake(&self, type_: u8, detail: u8, x: i16, y: i16) -> Result<(), error::Error> {
        self.connection
            .xtest_fake_input(type_, detail, x11rb::CURRENT_TIME, self.root, x, y, 0)?;
        Ok(())
    }

    fn release(&mut self) -> Result<(), error::Error> {
        for key in std::mem::take(&mut self.pressed_keys) {
            self.fake(x11rb::protocol::xproto::KEY_RELEASE_EVENT, key, 0, 0)?;
        }
        for button in std::mem::take(&mut self.pressed_buttons) {
            self.fake(x11rb::protocol::xproto::BUTTON_RELEASE_EVENT, button, 0, 0)?;
        }
        Ok(())
    }
}

impl<C: Connection> Drop for Injector<'_, C> {
    fn drop(&mut self) {
        let _ = self.release();
        let _ = self.connection.flush();
    }
}
