use x11rb::connection::Connection;
use x11rb::protocol::randr::ConnectionExt as _;
use x11rb::protocol::xproto::ConnectionExt as _;
use x11rb::wrapper::ConnectionExt as _;

const ALL_DESKTOPS: u32 = 0xffffffff;
const MOD1_INDEX: usize = 3;
const MILLIMETERS_PER_INCH: f32 = 25.4;
const KEYSYM_ESCAPE: u32 = 0xff1b;

x11rb::atom_manager! {
    Atoms: AtomsCookie {
        UTF8_STRING,
        _NET_ACTIVE_WINDOW,
        _NET_CLIENT_LIST,
        _NET_CLIENT_LIST_STACKING,
        _NET_CURRENT_DESKTOP,
        _NET_WM_DESKTOP,
        _NET_WM_ICON,
        _NET_WM_NAME,
        _NET_WM_STATE,
        _NET_WM_STATE_ABOVE,
        _NET_WM_STATE_HIDDEN,
        _NET_WM_STATE_SKIP_TASKBAR,
        _NET_WM_WINDOW_TYPE,
        _NET_WM_WINDOW_TYPE_DIALOG,
        _NET_WM_WINDOW_TYPE_NORMAL,
    }
}

#[derive(Clone, Copy, Debug)]
struct Area {
    x: i32,
    y: i32,
    width: i32,
    height: i32,
}

struct Window {
    id: x11rb::protocol::xproto::Window,
    area: Area,
    title: String,
    selectable: bool,
    misplaced: bool,
}

fn property(
    connection: &impl Connection,
    window: x11rb::protocol::xproto::Window,
    atom: x11rb::protocol::xproto::Atom,
) -> Result<Vec<u32>, error::Error> {
    let reply = connection
        .get_property(
            false,
            window,
            atom,
            x11rb::protocol::xproto::AtomEnum::ANY,
            0,
            u32::MAX,
        )?
        .reply()?;
    Ok(reply
        .value32()
        .map(|values| values.collect())
        .unwrap_or_default())
}

fn monitor_of(monitors: &[Area], area: Area) -> Option<usize> {
    let overlap = |monitor: &Area| {
        let width = (area.x + area.width).min(monitor.x + monitor.width) - area.x.max(monitor.x);
        let height = (area.y + area.height).min(monitor.y + monitor.height) - area.y.max(monitor.y);
        width.max(0) as i64 * height.max(0) as i64
    };
    (0..monitors.len())
        .filter(|&index| overlap(&monitors[index]) > 0)
        .max_by_key(|&index| overlap(&monitors[index]))
}

pub fn run(
    connection: &impl Connection,
    root: x11rb::protocol::xproto::Window,
    key: u8,
    reverse: bool,
    linear: bool,
) -> crate::Return {
    let atoms = Atoms::new(connection)?.reply()?;
    let outputs = connection.randr_get_monitors(root, true)?.reply()?.monitors;
    let monitors = outputs
        .iter()
        .map(|monitor| Area {
            x: monitor.x as i32,
            y: monitor.y as i32,
            width: monitor.width as i32,
            height: monitor.height as i32,
        })
        .collect::<Vec<_>>();
    let active = property(connection, root, atoms._NET_ACTIVE_WINDOW)?
        .first()
        .copied()
        .unwrap_or(x11rb::NONE);
    let current_desktop = property(connection, root, atoms._NET_CURRENT_DESKTOP)?
        .first()
        .copied();

    let describe = |id| -> Result<Window, error::Error> {
        let geometry = connection.get_geometry(id)?.reply()?;
        let position = connection.translate_coordinates(id, root, 0, 0)?.reply()?;
        let desktop = property(connection, id, atoms._NET_WM_DESKTOP)?
            .first()
            .copied();
        let types = property(connection, id, atoms._NET_WM_WINDOW_TYPE)?;
        let states = property(connection, id, atoms._NET_WM_STATE)?;
        let name = connection
            .get_property(
                false,
                id,
                atoms._NET_WM_NAME,
                atoms.UTF8_STRING,
                0,
                u32::MAX,
            )?
            .reply()?
            .value;
        let name = if name.is_empty() {
            connection
                .get_property(
                    false,
                    id,
                    x11rb::protocol::xproto::AtomEnum::WM_NAME,
                    x11rb::protocol::xproto::AtomEnum::ANY,
                    0,
                    u32::MAX,
                )?
                .reply()?
                .value
        } else {
            name
        };
        Ok(Window {
            id,
            area: Area {
                x: position.dst_x as i32,
                y: position.dst_y as i32,
                width: geometry.width as i32,
                height: geometry.height as i32,
            },
            title: String::from_utf8_lossy(&name).into_owned(),
            selectable: (desktop == current_desktop || desktop == Some(ALL_DESKTOPS))
                && (types.is_empty()
                    || types.contains(&atoms._NET_WM_WINDOW_TYPE_NORMAL)
                    || types.contains(&atoms._NET_WM_WINDOW_TYPE_DIALOG))
                && !states.contains(&atoms._NET_WM_STATE_SKIP_TASKBAR),
            misplaced: states.contains(&atoms._NET_WM_STATE_ABOVE)
                || states.contains(&atoms._NET_WM_STATE_HIDDEN),
        })
    };
    let mut windows = property(connection, root, atoms._NET_CLIENT_LIST_STACKING)?
        .into_iter()
        .rev()
        // A window closing between the requests fails them with BadWindow.
        .filter_map(|id| describe(id).ok())
        .collect::<Vec<_>>();

    let area = match windows.iter().find(|window| window.id == active) {
        Some(window) => window.area,
        None => {
            let pointer = connection.query_pointer(root)?.reply()?;
            Area {
                x: pointer.root_x as i32,
                y: pointer.root_y as i32,
                width: 1,
                height: 1,
            }
        }
    };
    let Some(monitor) = monitor_of(&monitors, area) else {
        error::bail!("no monitor shows {area:?}");
    };
    windows
        .retain(|window| window.selectable && monitor_of(&monitors, window.area) == Some(monitor));
    if linear {
        let order = property(connection, root, atoms._NET_CLIENT_LIST)?;
        windows.sort_by_key(|window| order.iter().position(|&id| id == window.id));
    } else {
        // Neither always-on-top nor minimized windows move in the stacking order when used.
        windows.sort_by_key(|window| (window.id != active, window.misplaced));
    }
    if windows.iter().all(|window| window.id == active) {
        return Ok(());
    }

    let status = connection
        .grab_keyboard(
            false,
            root,
            x11rb::CURRENT_TIME,
            x11rb::protocol::xproto::GrabMode::ASYNC,
            x11rb::protocol::xproto::GrabMode::ASYNC,
        )?
        .reply()?
        .status;
    if status != x11rb::protocol::xproto::GrabStatus::SUCCESS {
        error::bail!("failed to grab the keyboard: {status:?}");
    }

    let length = windows.len();
    let current = windows.iter().position(|window| window.id == active);
    let mut selected = if reverse {
        current.map_or(length - 1, |index| (index + length - 1) % length)
    } else {
        current.map_or(0, |index| (index + 1) % length)
    };
    let mask = connection.query_pointer(root)?.reply()?.mask;
    let committed = if mask.contains(x11rb::protocol::xproto::KeyButMask::MOD1) {
        let setup = connection.setup();
        let mapping = connection
            .get_keyboard_mapping(setup.min_keycode, setup.max_keycode - setup.min_keycode + 1)?
            .reply()?;
        let keysym = |keycode: u8| {
            mapping.keysyms
                [(keycode - setup.min_keycode) as usize * mapping.keysyms_per_keycode as usize]
        };
        let modifiers = connection.get_modifier_mapping()?.reply()?;
        let alt_keycodes = modifiers
            .keycodes
            .chunks(modifiers.keycodes_per_modifier() as usize)
            .nth(MOD1_INDEX)
            .unwrap_or_default();
        let bounds = monitors[monitor];
        let icons = windows
            .iter()
            .map(|window| property(connection, window.id, atoms._NET_WM_ICON).unwrap_or_default())
            .collect::<Vec<_>>();
        let rows = windows
            .iter()
            .zip(&icons)
            .map(|(window, icon)| (window.title.as_str(), icon.as_slice()))
            .collect::<Vec<_>>();
        let dots_per_inch = bounds.width as f32 * MILLIMETERS_PER_INCH
            / outputs[monitor].width_in_millimeters as f32;
        let picker = super::picker::Picker::new(dots_per_inch, rows, bounds.width / 3)?;

        let window = connection.generate_id()?;
        connection.create_window(
            x11rb::COPY_DEPTH_FROM_PARENT,
            window,
            root,
            (bounds.x + (bounds.width - picker.width as i32) / 2) as i16,
            (bounds.y + (bounds.height - picker.height as i32) / 2) as i16,
            picker.width,
            picker.height,
            1,
            x11rb::protocol::xproto::WindowClass::INPUT_OUTPUT,
            x11rb::COPY_FROM_PARENT,
            &x11rb::protocol::xproto::CreateWindowAux::new()
                .border_pixel(super::BORDER)
                .override_redirect(1)
                .event_mask(x11rb::protocol::xproto::EventMask::EXPOSURE),
        )?;
        let context = connection.generate_id()?;
        connection.create_gc(
            context,
            window,
            &x11rb::protocol::xproto::CreateGCAux::new(),
        )?;
        connection.map_window(window)?;
        connection.flush()?;

        let draw = |selected: usize| -> crate::Return {
            x11rb::image::Image::new(
                picker.width,
                picker.height,
                x11rb::image::ScanlinePad::Pad32,
                24,
                x11rb::image::BitsPerPixel::B32,
                x11rb::image::ImageOrder::LsbFirst,
                std::borrow::Cow::Owned(picker.render(selected)),
            )?
            .put(connection, window, context, 0, 0)?;
            connection.flush()?;
            Ok(())
        };
        let committed = loop {
            match connection.wait_for_event()? {
                x11rb::protocol::Event::Expose(_) => draw(selected)?,
                x11rb::protocol::Event::KeyPress(event) => match keysym(event.detail) {
                    _ if event.detail == key => {
                        let shift = event
                            .state
                            .contains(x11rb::protocol::xproto::KeyButMask::SHIFT);
                        selected = if shift {
                            (selected + windows.len() - 1) % windows.len()
                        } else {
                            (selected + 1) % windows.len()
                        };
                        draw(selected)?;
                    }
                    KEYSYM_ESCAPE => break false,
                    _ => {}
                },
                x11rb::protocol::Event::KeyRelease(event)
                    if alt_keycodes.contains(&event.detail) =>
                {
                    break true;
                }
                _ => {}
            }
        };
        connection.free_gc(context)?;
        connection.destroy_window(window)?;
        committed
    } else {
        true
    };
    connection.ungrab_keyboard(x11rb::CURRENT_TIME)?;
    if !committed {
        connection.flush()?;
        return Ok(());
    }

    connection.send_event(
        false,
        root,
        x11rb::protocol::xproto::EventMask::SUBSTRUCTURE_REDIRECT
            | x11rb::protocol::xproto::EventMask::SUBSTRUCTURE_NOTIFY,
        x11rb::protocol::xproto::ClientMessageEvent::new(
            32,
            windows[selected].id,
            atoms._NET_ACTIVE_WINDOW,
            [2, x11rb::CURRENT_TIME, active, 0, 0],
        ),
    )?;
    connection.sync()?;
    Ok(())
}
