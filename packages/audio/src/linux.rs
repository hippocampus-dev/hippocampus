use error::context::Context;

pub fn prepare_loopback(device_name: String, rate: u32, channels: u8) -> Result<(), error::Error> {
    let mut mainloop = libpulse_binding::mainloop::standard::Mainloop::new()
        .ok_or(error::error!("error occurred while creating mainloop"))?;
    let mut context = libpulse_binding::context::Context::new(&mainloop, &device_name)
        .ok_or(error::error!("error occurred while creating context"))?;

    context
        .connect(None, libpulse_binding::context::FlagSet::NOFLAGS, None)
        .context("error occurred while connecting to context")?;

    loop {
        match mainloop.iterate(false) {
            libpulse_binding::mainloop::standard::IterateResult::Quit(_)
            | libpulse_binding::mainloop::standard::IterateResult::Err(_) => {
                return Err(error::error!("error occurred while iterating mainloop"));
            }
            libpulse_binding::mainloop::standard::IterateResult::Success(_) => {}
        }

        match context.get_state() {
            libpulse_binding::context::State::Ready => break,
            libpulse_binding::context::State::Failed
            | libpulse_binding::context::State::Terminated => {
                return Err(error::error!("error occurred while connecting to context"));
            }
            _ => std::thread::sleep(std::time::Duration::from_millis(10)),
        }
    }

    let virtual_sink_exists = std::rc::Rc::new(std::cell::Cell::new(false));
    let loopback_exists = std::rc::Rc::new(std::cell::Cell::new(false));
    let default_sink_name = std::rc::Rc::new(std::cell::RefCell::new(String::new()));

    let mut introspector = context.introspect();

    let cloned_default_sink_name = std::rc::Rc::clone(&default_sink_name);
    wait_for_operation(
        &mut mainloop,
        &introspector.get_server_info(move |info| {
            if let Some(ref sink) = info.default_sink_name {
                cloned_default_sink_name.borrow_mut().push_str(sink);
            }
        }),
    )?;

    let cloned_device_name = device_name.clone();
    let cloned_virtual_sink_exists = std::rc::Rc::clone(&virtual_sink_exists);
    let cloned_loopback_exists = std::rc::Rc::clone(&loopback_exists);
    let cloned_default_sink_name = std::rc::Rc::clone(&default_sink_name);
    wait_for_operation(
        &mut mainloop,
        &introspector.get_module_info_list(move |list| {
            if let libpulse_binding::callbacks::ListResult::Item(info) = list
                && let Some(name) = &info.name
            {
                if name == "module-null-sink"
                    && info
                        .argument
                        .as_ref()
                        .is_some_and(|arg| arg.contains(&format!("sink_name={cloned_device_name}")))
                {
                    cloned_virtual_sink_exists.set(true);
                } else if name == "module-loopback"
                    && info.argument.as_ref().is_some_and(|arg| {
                        arg.contains(&format!("sink={cloned_device_name}"))
                            && arg.contains(&format!(
                                "source={}.monitor",
                                cloned_default_sink_name.borrow()
                            ))
                    })
                {
                    cloned_loopback_exists.set(true);
                }
            }
        }),
    )?;

    if !virtual_sink_exists.get() {
        wait_for_operation(
            &mut mainloop,
            &introspector.load_module(
                "module-null-sink",
                &format!("sink_name={}", &device_name),
                |_i| {},
            ),
        )?;
    }

    if !loopback_exists.get() {
        wait_for_operation(
            &mut mainloop,
            &introspector.load_module(
                "module-loopback",
                &format!(
                    "source={}.monitor sink={} rate={} channels={}",
                    default_sink_name.borrow(),
                    &device_name,
                    rate,
                    channels,
                ),
                |_i| {},
            ),
        )?;
    }

    context.disconnect();

    while context.get_state() != libpulse_binding::context::State::Terminated {
        match mainloop.iterate(false) {
            libpulse_binding::mainloop::standard::IterateResult::Quit(_)
            | libpulse_binding::mainloop::standard::IterateResult::Err(_) => break,
            libpulse_binding::mainloop::standard::IterateResult::Success(_) => {}
        }
    }

    Ok(())
}

fn wait_for_operation<ClosureProto: ?Sized>(
    mainloop: &mut libpulse_binding::mainloop::standard::Mainloop,
    op: &libpulse_binding::operation::Operation<ClosureProto>,
) -> Result<(), error::Error> {
    while op.get_state() == libpulse_binding::operation::State::Running {
        match mainloop.iterate(false) {
            libpulse_binding::mainloop::standard::IterateResult::Quit(_)
            | libpulse_binding::mainloop::standard::IterateResult::Err(_) => {
                return Err(error::error!("error occurred while iterating mainloop"));
            }
            libpulse_binding::mainloop::standard::IterateResult::Success(_) => {}
        }
    }

    Ok(())
}

pub fn capture_device<S, F>(
    device_name: S,
    rate: u32,
    channels: u8,
    mut callback: F,
) -> Result<(), error::Error>
where
    S: AsRef<str>,
    F: FnMut(&[u8]) -> crate::CaptureControl,
{
    let buffer_size = rate * (channels as u32) * 2;
    let mut buffer = vec![0u8; buffer_size as usize];

    let simple = libpulse_simple_binding::Simple::new(
        None,
        "audio_capture",
        libpulse_binding::stream::Direction::Record,
        Some(&format!("{}.monitor", device_name.as_ref())),
        "capture",
        &libpulse_binding::sample::Spec {
            format: libpulse_binding::sample::Format::S16le,
            channels,
            rate,
        },
        None,
        None,
    )?;

    loop {
        simple.read(&mut buffer)?;

        match callback(&buffer) {
            crate::CaptureControl::Continue => {}
            crate::CaptureControl::Stop => break,
        }
    }

    Ok(())
}
