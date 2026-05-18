use error::context::Context;
use std::convert::TryInto;
use std::fmt::Write;
use std::io::Read;

mod skel {
    include!("https/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for https_bss_types::event {}

#[derive(Debug)]
pub struct SymbolNotFoundError;

impl std::error::Error for SymbolNotFoundError {}
impl std::fmt::Display for SymbolNotFoundError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "symbol not found error")
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
enum FrameType {
    DATA,
    HEADERS,
    UNASSIGNED,
}

pub fn watch(
    args: crate::Args,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    shutdown: std::sync::Arc<std::sync::atomic::AtomicBool>,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), error::Error> {
    let mut builder = HttpsSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;
    open.rodata().tool_config.tgid = args.tgid.unwrap_or_default();

    let mut file = std::fs::File::open(&args.libssl_path)
        .map_err(|e| error::Error::from_message(e.to_string()))
        .context(format!(
            "error occurred while reading {}",
            &args.libssl_path.display()
        ))?;
    let mut v: Vec<u8> = Vec::new();
    let _ = file.read_to_end(&mut v);
    let e = elf::parse(&v)?;

    let mut load = open.load()?;
    let ssl_read_offset = match e.symbol_table.get("SSL_read") {
        Some(elf::symbol::Symbol::Symbol32(symbol)) => symbol.value as usize,
        Some(elf::symbol::Symbol::Symbol64(symbol)) => symbol.value as usize,
        _ => return Err(SymbolNotFoundError.into()),
    };
    let enter_read_link = load.progs_mut().enter_ssl_read().attach_uprobe(
        false,
        -1,
        &args.libssl_path,
        ssl_read_offset,
    )?;
    let exit_read_link = load.progs_mut().exit_ssl_read().attach_uprobe(
        true,
        -1,
        &args.libssl_path,
        ssl_read_offset,
    )?;
    let ssl_write_offset = match e.symbol_table.get("SSL_write") {
        Some(elf::symbol::Symbol::Symbol32(symbol)) => symbol.value as usize,
        Some(elf::symbol::Symbol::Symbol64(symbol)) => symbol.value as usize,
        _ => return Err(SymbolNotFoundError.into()),
    };
    let enter_write_link = load.progs_mut().enter_ssl_write().attach_uprobe(
        false,
        -1,
        &args.libssl_path,
        ssl_write_offset,
    )?;
    let exit_write_link = load.progs_mut().exit_ssl_write().attach_uprobe(
        true,
        -1,
        &args.libssl_path,
        ssl_write_offset,
    )?;
    load.links = crate::https::HttpsLinks {
        enter_ssl_read: Some(enter_read_link),
        exit_ssl_read: Some(exit_read_link),
        enter_ssl_write: Some(enter_write_link),
        exit_ssl_write: Some(exit_write_link),
    };

    let buffer = libbpf_rs::PerfBufferBuilder::new(load.maps_mut().events())
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = https_bss_types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");
            let len = std::cmp::min(event.len as usize, event.buf.len());
            let original = &event.buf[0..len];
            let mut buffer = original;
            if buffer.len() >= 24 {
                if let Ok(s) = std::str::from_utf8(&buffer[0..24]) {
                    if s == "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n" {
                        buffer = &buffer[24..buffer.len()];
                    }
                };
            }
            let mut payload = String::new();
            while buffer.len() >= 9 {
                let solicit_header = if let Ok(header) = &buffer[0..9].try_into() {
                    solicit::http::frame::unpack_header(header)
                } else {
                    break;
                };
                if buffer.len() < solicit_header.0 as usize {
                    break;
                }
                let frame_type = match solicit_header.1 {
                    0u8 => FrameType::DATA,
                    1u8 => FrameType::HEADERS,
                    _ => FrameType::UNASSIGNED,
                };
                if frame_type == FrameType::UNASSIGNED {
                    break;
                }
                if frame_type == FrameType::HEADERS {
                    let mut decoder = hpack::Decoder::new();
                    if let Ok(headers) = decoder.decode(&buffer[9..(9 + solicit_header.0) as usize])
                    {
                        let kv = headers.iter().filter_map(|header| {
                            Some((
                                std::str::from_utf8(&header.0).ok()?,
                                std::str::from_utf8(&header.1).ok()?,
                            ))
                        });
                        for (key, value) in kv {
                            writeln!(payload, "{key}: {value}").unwrap();
                        }
                        payload.push('\n');
                    }
                }
                if frame_type == FrameType::DATA {
                    if let Ok(s) = std::str::from_utf8(&buffer[9..(9 + solicit_header.0) as usize])
                    {
                        payload.push_str(s.trim_end());
                    }
                }
                buffer = &buffer[(9 + solicit_header.0) as usize..buffer.len()];
            }
            if !payload.is_empty() {
                tx.send(crate::Event::Result(crate::ResultEvent::L7(payload)))
                    .unwrap();
            }
            if buffer.is_empty() {
                return;
            }
            let mut headers = [httparse::EMPTY_HEADER; 64];
            match event.kind {
                https_bss_types::kind::request => {
                    let mut request = httparse::Request::new(&mut headers);
                    if request.parse(original).is_ok() {
                        if let Ok(s) = std::str::from_utf8(original) {
                            let payload = s.trim_end().to_string();
                            tx.send(crate::Event::Result(crate::ResultEvent::L7(payload)))
                                .unwrap();
                        };
                    }
                }
                https_bss_types::kind::response => {
                    let mut response = httparse::Response::new(&mut headers);
                    if response.parse(original).is_ok() {
                        if let Ok(s) = std::str::from_utf8(original) {
                            let payload = s.trim_end().to_string();
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
