mod skel;

unsafe impl plain::Plain for skel::connect_bss_types::event {}
unsafe impl plain::Plain for skel::connect_bss_types::arg {}

pub fn watch(
    args: crate::Args,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let mut builder = skel::ConnectSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;
    open.rodata().tool_config.tgid = args.pid.unwrap_or_default();

    let mut load = open.load()?;
    load.attach()?;

    let (event_tx, event_rx): (
        std::sync::mpsc::Sender<i32>,
        std::sync::mpsc::Receiver<i32>,
    ) = std::sync::mpsc::channel();

    let buffer = libbpf_rs::PerfBufferBuilder::new(load.maps_mut().events())
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = skel::connect_bss_types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");
            println!("{}, addr: {}, port: {}", event.fd, std::net::Ipv4Addr::from(u32::swap_bytes(event.sin_addr)), event.sin_port);
            event_tx.send(event.fd);
        })
        .lost_cb(|cpu: i32, count: u64| {
            eprintln!("Lost {} events on CPU {}", count, cpu);
        })
        .build()?;

    loop {
        buffer.poll(std::time::Duration::from_millis(100))?;

        let mut map = load.maps_mut();
        let fds = map.fds();
        if let Ok(fd) = event_rx.try_recv() {
            let key = fd.to_ne_bytes();
            if let Ok(Some(data)) = fds.lookup(&key, libbpf_rs::MapFlags::empty()) {
                let mut arg = skel::connect_bss_types::arg::default();
                plain::copy_from_bytes(&mut arg, &data).expect("Data buffer was too short");
                println!("{:?}", &arg);
                arg.closed = 1;
                let new = unsafe { plain::as_bytes(&arg) };
                fds.update(&key, new, libbpf_rs::MapFlags::empty()).unwrap();
            }
        }
    }
}
