use x11rb::connection::Connection;
use x11rb::protocol::xproto::ConnectionExt as _;
use x11rb::wrapper::ConnectionExt as _;

x11rb::atom_manager! {
    pub Atoms: AtomsCookie {
        CLIPBOARD,
        UTF8_STRING,
        TARGETS,
        TELEPUT,
    }
}

// X11 button numbers and the Linux input-event codes libei takes (linux/input-event-codes.h).
pub const BUTTONS: [(u8, u32); 5] = [(1, 0x110), (2, 0x112), (3, 0x111), (8, 0x113), (9, 0x114)];

pub fn read(
    connection: &impl Connection,
    window: x11rb::protocol::xproto::Window,
    atoms: &Atoms,
    event: &x11rb::protocol::xproto::SelectionNotifyEvent,
) -> Result<Option<String>, error::Error> {
    let property = connection
        .get_property(
            true,
            window,
            event.property,
            x11rb::protocol::xproto::AtomEnum::ANY,
            0,
            u32::MAX,
        )?
        .reply()?;
    if property.type_ != atoms.UTF8_STRING {
        return Ok(None);
    }
    Ok(Some(String::from_utf8_lossy(&property.value).into_owned()))
}

pub fn answer(
    connection: &impl Connection,
    window: x11rb::protocol::xproto::Window,
    atoms: &Atoms,
    offered: &str,
    event: &x11rb::protocol::xproto::SelectionRequestEvent,
) -> Result<(), error::Error> {
    // An obsolete requestor leaves the property None: https://x.org/releases/X11R7.6/doc/xorg-docs/specs/ICCCM/icccm.html#requests
    let property = if event.property == x11rb::NONE {
        event.target
    } else {
        event.property
    };
    let property = if event.requestor == window {
        x11rb::NONE
    } else if event.target == atoms.TARGETS {
        connection.change_property32(
            x11rb::protocol::xproto::PropMode::REPLACE,
            event.requestor,
            property,
            x11rb::protocol::xproto::AtomEnum::ATOM,
            &[atoms.TARGETS, atoms.UTF8_STRING],
        )?;
        property
    } else if event.target == atoms.UTF8_STRING {
        match connection.change_property8(
            x11rb::protocol::xproto::PropMode::REPLACE,
            event.requestor,
            property,
            atoms.UTF8_STRING,
            offered.as_bytes(),
        ) {
            Ok(_) => property,
            Err(x11rb::errors::ConnectionError::MaximumRequestLengthExceeded) => x11rb::NONE,
            Err(e) => return Err(e.into()),
        }
    } else {
        x11rb::NONE
    };
    connection.send_event(
        false,
        event.requestor,
        x11rb::protocol::xproto::EventMask::NO_EVENT,
        x11rb::protocol::xproto::SelectionNotifyEvent {
            response_type: x11rb::protocol::xproto::SELECTION_NOTIFY_EVENT,
            sequence: 0,
            time: event.time,
            requestor: event.requestor,
            selection: event.selection,
            target: event.target,
            property,
        },
    )?;
    connection.flush()?;
    Ok(())
}
