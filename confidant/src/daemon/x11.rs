use x11rb::connection::Connection;
use x11rb::protocol::xkb::ConnectionExt as _;
use x11rb::protocol::xproto::ConnectionExt as _;

pub fn run(args: crate::Args) -> crate::Return {
    let (connection, screen_number) = x11rb::connect(None)?;
    let root = connection.setup().roots[screen_number].root;
    connection.xkb_use_extension(1, 0)?.reply()?;
    // X11 autorepeat otherwise delivers a held key as repeated release/press pairs.
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
    let confine_modifiers = args.confine_modifier.iter().fold(
        x11rb::protocol::xproto::ModMask::from(0u16),
        |mask, modifier| {
            mask | match modifier {
                crate::Modifier::Shift => x11rb::protocol::xproto::ModMask::SHIFT,
                crate::Modifier::Control => x11rb::protocol::xproto::ModMask::CONTROL,
                crate::Modifier::Alt => x11rb::protocol::xproto::ModMask::M1,
                crate::Modifier::Super => x11rb::protocol::xproto::ModMask::M4,
            }
        },
    );
    let switch_modifiers =
        x11rb::protocol::xproto::ModMask::CONTROL | x11rb::protocol::xproto::ModMask::M1;
    // A passive grab matches the modifiers exactly, Caps Lock and Num Lock included: https://www.x.org/releases/X11R7.7/doc/xproto/x11protocol.html#requests:GrabKey
    for locks in [
        x11rb::protocol::xproto::ModMask::from(0u16),
        x11rb::protocol::xproto::ModMask::LOCK,
        x11rb::protocol::xproto::ModMask::M2,
        x11rb::protocol::xproto::ModMask::LOCK | x11rb::protocol::xproto::ModMask::M2,
    ] {
        for (key, modifiers) in [
            (args.switch_key, switch_modifiers),
            (
                args.switch_key,
                switch_modifiers | x11rb::protocol::xproto::ModMask::SHIFT,
            ),
            (args.confine_key, confine_modifiers),
        ] {
            connection
                .grab_key(
                    false,
                    root,
                    modifiers | locks,
                    key,
                    x11rb::protocol::xproto::GrabMode::ASYNC,
                    x11rb::protocol::xproto::GrabMode::ASYNC,
                )?
                .check()?;
        }
    }

    let mut barriers = Vec::new();
    loop {
        match connection.wait_for_event()? {
            x11rb::protocol::Event::KeyPress(event) if event.detail == args.switch_key => {
                crate::switch::x11::run(
                    &connection,
                    root,
                    args.switch_key,
                    event
                        .state
                        .contains(x11rb::protocol::xproto::KeyButMask::SHIFT),
                    args.linear,
                )?;
            }
            // DETECTABLE_AUTO_REPEAT repeats a held key as KeyPress alone, so the confinement flips on its release.
            x11rb::protocol::Event::KeyRelease(event) if event.detail == args.confine_key => {
                crate::confine::x11::run(&connection, root, &mut barriers)?;
            }
            _ => {}
        }
    }
}
