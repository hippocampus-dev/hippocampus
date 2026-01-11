use libbpf_rs::skel::OpenSkel;
use libbpf_rs::skel::Skel;
use libbpf_rs::skel::SkelBuilder;
use std::os::unix::ffi::OsStrExt;

mod skel {
    include!("bpf/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for types::event {}

fn get_host_pid() -> u32 {
    if let Ok(status) = std::fs::read_to_string("/proc/self/status") {
        for line in status.lines() {
            if line.starts_with("NSpid:") {
                if let Some(first_pid) = line.split_whitespace().nth(1) {
                    if let Ok(pid) = first_pid.parse::<u32>() {
                        return pid;
                    }
                }
            }
        }
    }
    std::process::id()
}

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
    open.maps.rodata_data.tool_config.this = get_host_pid();

    let mut load = open.load()?;
    load.attach()?;

    let buffer = libbpf_rs::PerfBufferBuilder::new(&load.maps.events)
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");

            let pathname = std::ffi::CStr::from_bytes_until_nul(&event.pathname)
                .ok()
                .and_then(|c| c.to_str().ok())
                .unwrap_or("");

            if !pathname.is_empty() {
                if let Err(e) = tx.send(pathname.to_string()) {
                    eprintln!("Failed to send message: {}", e);
                }
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
