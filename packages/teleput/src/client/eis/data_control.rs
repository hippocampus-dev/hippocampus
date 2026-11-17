use std::os::fd::AsFd;
use std::os::fd::AsRawFd;
use tokio::io::AsyncReadExt;
use wayland_client::Proxy;

const OWNER_MIME_TYPE: &str = "application/x-teleput";

pub struct DataControl {
    connection: wayland_client::Connection,
    queue: wayland_client::EventQueue<State>,
    state: State,
    readiness: tokio::io::unix::AsyncFd<std::os::fd::RawFd>,
    manager: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_manager_v1::ZwlrDataControlManagerV1,
    device: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::ZwlrDataControlDeviceV1,
}

struct State {
    read_sender: tokio::sync::mpsc::UnboundedSender<Result<String, std::io::Error>>,
    selection: Option<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::ZwlrDataControlOfferV1>,
    unread: bool,
}

impl DataControl {
    pub fn connect(
        read_sender: tokio::sync::mpsc::UnboundedSender<Result<String, std::io::Error>>,
    ) -> Result<Option<Self>, error::Error> {
        let connection = wayland_client::Connection::connect_to_env()?;
        let (globals, mut queue) =
            wayland_client::globals::registry_queue_init::<State>(&connection)?;
        let manager = match globals.bind::<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_manager_v1::ZwlrDataControlManagerV1, _, _>(&queue.handle(), 1..=2, ()) {
            Ok(manager) => manager,
            Err(wayland_client::globals::BindError::NotPresent) => return Ok(None),
            Err(e) => return Err(e.into()),
        };
        let seat = globals.bind::<wayland_client::protocol::wl_seat::WlSeat, _, _>(
            &queue.handle(),
            1..=1,
            (),
        )?;
        let device = manager.get_data_device(&seat, &queue.handle(), ());
        let mut state = State {
            read_sender,
            selection: None,
            unread: false,
        };
        // The first selection event is sent upon binding: https://wayland.app/protocols/wlr-data-control-unstable-v1#zwlr_data_control_device_v1:event:selection
        queue.roundtrip(&mut state)?;
        state.unread = false;
        let readiness = tokio::io::unix::AsyncFd::with_interest(
            connection.backend().poll_fd().as_raw_fd(),
            tokio::io::Interest::READABLE,
        )?;
        Ok(Some(Self {
            connection,
            queue,
            state,
            readiness,
            manager,
            device,
        }))
    }

    pub async fn dispatch(&mut self) -> crate::Return {
        self.queue.dispatch_pending(&mut self.state)?;
        self.state.read();
        self.connection.flush()?;
        if let Some(guard) = self.queue.prepare_read() {
            let mut ready = self.readiness.readable().await?;
            match guard.read() {
                Ok(_) => {}
                Err(wayland_client::backend::WaylandError::Io(e))
                    if e.kind() == std::io::ErrorKind::WouldBlock =>
                {
                    ready.clear_ready()
                }
                Err(e) => return Err(e.into()),
            }
        }
        self.queue.dispatch_pending(&mut self.state)?;
        self.state.read();
        self.connection.flush()?;
        Ok(())
    }

    pub fn set(&self, text: String) -> crate::Return {
        let source = self.manager.create_data_source(&self.queue.handle(), text);
        source.offer(super::TEXT_MIME_TYPE.to_string());
        source.offer(OWNER_MIME_TYPE.to_string());
        self.device.set_selection(Some(&source));
        self.connection.flush()?;
        Ok(())
    }
}

impl State {
    fn read(&mut self) {
        if !std::mem::take(&mut self.unread) {
            return;
        }
        let Some(offer) = &self.selection else {
            return;
        };
        let (reader, writer) = match rustix::pipe::pipe_with(rustix::pipe::PipeFlags::CLOEXEC) {
            Ok(pipe) => pipe,
            Err(e) => {
                eprintln!("clipboard not read: {e}");
                return;
            }
        };
        offer.receive(super::TEXT_MIME_TYPE.to_string(), writer.as_fd());
        let read_sender = self.read_sender.clone();
        tokio::spawn(async move {
            let read = async {
                let mut text = String::new();
                tokio::net::unix::pipe::Receiver::from_owned_fd(reader)?
                    .read_to_string(&mut text)
                    .await?;
                Ok(text)
            };
            let _ = read_sender.send(read.await);
        });
    }
}

impl
    wayland_client::Dispatch<
        wayland_client::protocol::wl_registry::WlRegistry,
        wayland_client::globals::GlobalListContents,
    > for State
{
    fn event(
        _: &mut Self,
        _: &wayland_client::protocol::wl_registry::WlRegistry,
        _: wayland_client::protocol::wl_registry::Event,
        _: &wayland_client::globals::GlobalListContents,
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
    }
}

impl wayland_client::Dispatch<wayland_client::protocol::wl_seat::WlSeat, ()> for State {
    fn event(
        _: &mut Self,
        _: &wayland_client::protocol::wl_seat::WlSeat,
        _: wayland_client::protocol::wl_seat::Event,
        _: &(),
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
    }
}

impl wayland_client::Dispatch<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_manager_v1::ZwlrDataControlManagerV1, ()> for State {
    fn event(
        _: &mut Self,
        _: &wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_manager_v1::ZwlrDataControlManagerV1,
        _: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_manager_v1::Event,
        _: &(),
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
    }
}

impl wayland_client::Dispatch<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::ZwlrDataControlDeviceV1, ()> for State {
    fn event(
        state: &mut Self,
        _: &wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::ZwlrDataControlDeviceV1,
        event: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::Event,
        _: &(),
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
        match event {
            wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::Event::Selection { id } => {
                state.unread = id.as_ref().is_some_and(|offer| {
                    offer
                        .data::<std::sync::Mutex<Vec<String>>>()
                        .is_some_and(|mime_types| {
                            let mime_types = mime_types.lock().unwrap();
                            mime_types.iter().any(|mime_type| mime_type == super::TEXT_MIME_TYPE)
                                && !mime_types.iter().any(|mime_type| mime_type == OWNER_MIME_TYPE)
                        })
                });
                if let Some(previous) = std::mem::replace(&mut state.selection, id) {
                    previous.destroy();
                }
            }
            wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::Event::PrimarySelection { id: Some(offer) } => {
                offer.destroy()
            }
            _ => {}
        }
    }

    wayland_client::event_created_child!(State, wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::ZwlrDataControlDeviceV1, [
        wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_device_v1::EVT_DATA_OFFER_OPCODE => (wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::ZwlrDataControlOfferV1, std::sync::Mutex::new(Vec::new())),
    ]);
}

impl wayland_client::Dispatch<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::ZwlrDataControlOfferV1, std::sync::Mutex<Vec<String>>> for State {
    fn event(
        _: &mut Self,
        _: &wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::ZwlrDataControlOfferV1,
        event: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::Event,
        mime_types: &std::sync::Mutex<Vec<String>>,
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
        if let wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_offer_v1::Event::Offer { mime_type } = event {
            mime_types.lock().unwrap().push(mime_type);
        }
    }
}

impl wayland_client::Dispatch<wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_source_v1::ZwlrDataControlSourceV1, String> for State {
    fn event(
        _: &mut Self,
        source: &wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_source_v1::ZwlrDataControlSourceV1,
        event: wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_source_v1::Event,
        text: &String,
        _: &wayland_client::Connection,
        _: &wayland_client::QueueHandle<Self>,
    ) {
        match event {
            wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_source_v1::Event::Send { fd, .. } => {
                let text = text.clone();
                tokio::task::spawn_blocking(move || {
                    if let Err(e) = std::io::Write::write_all(&mut std::fs::File::from(fd), text.as_bytes()) {
                        eprintln!("clipboard not written: {e}");
                    }
                });
            }
            wayland_protocols_wlr::data_control::v1::client::zwlr_data_control_source_v1::Event::Cancelled => source.destroy(),
            _ => {}
        }
    }
}
