mod skel {
    include!("cpu_usage/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for cpu_usage_bss_types::event {}

pub fn watch(
    args: crate::Args,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), error::Error> {
    let mut builder = CpuUsageSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;
    open.rodata().tool_config.tgid = args.tgid.unwrap_or_default();

    let mut load = open.load()?;
    load.attach()?;

    let mut hm: std::collections::HashMap<String, crate::core::types::Histogram> =
        std::collections::HashMap::new();
    loop {
        if stop.load(std::sync::atomic::Ordering::Relaxed) {
            std::thread::sleep(std::time::Duration::from_millis(100));
            continue;
        }
        let m = load.maps();
        let event = m.events();
        for key in event.keys() {
            if let Ok(Some(v)) = event.lookup(&key, libbpf_rs::MapFlags::empty()) {
                let mut event = cpu_usage_bss_types::event::default();
                plain::copy_from_bytes(&mut event, &v).expect("Data buffer was too short");
                let comm_str = if let Ok(s) = std::str::from_utf8(&event.comm) {
                    s.trim_end_matches(char::from(0))
                } else {
                    ""
                };
                hm.insert(
                    comm_str.to_string(),
                    crate::core::types::Histogram(event.histogram.to_vec()),
                );
            }
        }
        tx.send(crate::Event::Histogram(crate::HistogramEvent::CPUUsage(
            hm.clone(),
        )))?;
        std::thread::sleep(std::time::Duration::from_millis(100));
    }
}
