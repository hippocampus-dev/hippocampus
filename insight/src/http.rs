mod skel {
    include!("http/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for http_bss_types::event {}

pub fn watch(
    args: crate::Args,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    shutdown: std::sync::Arc<std::sync::atomic::AtomicBool>,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), error::Error> {
    let mut builder = HttpSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;
    open.rodata().tool_config.tgid = args.tgid.unwrap_or_default();

    let mut load = open.load()?;
    load.attach()?;

    let buffer = libbpf_rs::PerfBufferBuilder::new(load.maps_mut().events())
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = http_bss_types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");
            let mut headers = [httparse::EMPTY_HEADER; 64];
            match event.kind {
                http_bss_types::kind::request => {
                    let mut request = httparse::Request::new(&mut headers);
                    if request.parse(&event.buf).is_ok() {
                        if let Ok(s) = std::str::from_utf8(&event.buf) {
                            let trimmed = s.trim_end_matches(char::from(0)).trim_end();
                            let payload = trimmed.to_string();
                            tx.send(crate::Event::Result(crate::ResultEvent::L7(payload)))
                                .unwrap();
                        };
                    }
                }
                http_bss_types::kind::response => {
                    let mut response = httparse::Response::new(&mut headers);
                    if response.parse(&event.buf).is_ok() {
                        if let Ok(s) = std::str::from_utf8(&event.buf) {
                            let trimmed = s.trim_end_matches(char::from(0)).trim_end();
                            let payload = trimmed.to_string();
                            tx.send(crate::Event::Result(crate::ResultEvent::L7(payload)))
                                .unwrap();
                        };
                    }
                }
            }
        })
        .build()?;

    loop {
        if shutdown.load(std::sync::atomic::Ordering::Relaxed) {
            break;
        }
        if stop.load(std::sync::atomic::Ordering::Relaxed) {
            std::thread::sleep(std::time::Duration::from_millis(100));
            continue;
        }
        buffer.poll(std::time::Duration::from_millis(100))?;
    }

    Ok(())
}
