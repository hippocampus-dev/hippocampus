use libbpf_rs::skel::OpenSkel;
use libbpf_rs::skel::Skel;
use libbpf_rs::skel::SkelBuilder;
use std::os::unix::ffi::OsStrExt;

mod skel {
    include!("bpf/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for types::event {}

pub fn watch(
    directory: &std::path::PathBuf,
    tx: tokio::sync::mpsc::UnboundedSender<String>,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let builder = UnlinkSkelBuilder::default();
    let mut open_object = std::mem::MaybeUninit::uninit();
    let open = builder.open(&mut open_object)?;

    let mut bytes: [u8; 128] = [0; 128];
    bytes[..directory.as_os_str().len()].copy_from_slice(directory.as_os_str().as_bytes());
    open.maps.rodata_data.tool_config.directory = bytes;
    open.maps.rodata_data.tool_config.this = std::process::id();

    let mut load = open.load()?;
    load.attach()?;

    let buffer = libbpf_rs::PerfBufferBuilder::new(&load.maps.events)
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");

            let pathname = if let Ok(s) = std::str::from_utf8(&event.pathname) {
                s.trim_start_matches(char::from(0))
                    .trim_end_matches(char::from(0))
            } else {
                ""
            };

            if let Err(e) = tx.send(pathname.to_string()) {
                eprintln!("Failed to send message: {}", e);
            }
        })
        .lost_cb(|cpu: i32, count: u64| {
            eprintln!("Lost {} events on CPU {}", count, cpu);
        })
        .build()?;

    loop {
        if stop.load(std::sync::atomic::Ordering::Relaxed) {
            return Ok(());
        }
        buffer.poll(std::time::Duration::from_millis(100))?;
    }
}
