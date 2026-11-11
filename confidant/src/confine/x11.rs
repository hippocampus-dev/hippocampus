use x11rb::connection::Connection;
use x11rb::protocol::randr::ConnectionExt as _;
use x11rb::protocol::xfixes::ConnectionExt as _;
use x11rb::protocol::xproto::ConnectionExt as _;

pub fn run(
    connection: &impl Connection,
    root: x11rb::protocol::xproto::Window,
    barriers: &mut Vec<x11rb::protocol::xfixes::Barrier>,
) -> crate::Return {
    if !barriers.is_empty() {
        for barrier in barriers.drain(..) {
            connection.xfixes_delete_pointer_barrier(barrier)?;
        }
        connection.flush()?;
        return Ok(());
    }

    connection.xfixes_query_version(5, 0)?.reply()?;
    let pointer = connection.query_pointer(root)?.reply()?;
    let (x, y) = (pointer.root_x as i32, pointer.root_y as i32);
    let Some(monitor) = connection
        .randr_get_monitors(root, true)?
        .reply()?
        .monitors
        .into_iter()
        .find(|monitor| {
            (monitor.x as i32..monitor.x as i32 + monitor.width as i32).contains(&x)
                && (monitor.y as i32..monitor.y as i32 + monitor.height as i32).contains(&y)
        })
    else {
        error::bail!("no monitor shows the pointer at ({x}, {y})");
    };
    let (left, top) = (monitor.x as u16, monitor.y as u16);
    let (right, bottom) = (left + monitor.width, top + monitor.height);
    for (x1, y1, x2, y2) in [
        (left, top, right, top),
        (left, bottom, right, bottom),
        (left, top, left, bottom),
        (right, top, right, bottom),
    ] {
        let barrier = connection.generate_id()?;
        connection
            .xfixes_create_pointer_barrier(
                barrier,
                root,
                x1,
                y1,
                x2,
                y2,
                x11rb::protocol::xfixes::BarrierDirections::from(0u32),
                // Xorg fails XIAllMasterDevices with BadDevice.
                // Xorg applies a barrier without devices to all of them: https://gitlab.freedesktop.org/xorg/xserver/-/blob/master/Xi/xibarriers.c
                &[],
            )?
            .check()?;
        barriers.push(barrier);
    }
    Ok(())
}
