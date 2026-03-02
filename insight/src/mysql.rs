mod skel {
    include!("mysql/skel.rs");
}
use skel::*;

unsafe impl plain::Plain for mysql_bss_types::event {}

pub fn watch(
    args: crate::Args,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    shutdown: std::sync::Arc<std::sync::atomic::AtomicBool>,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), error::Error> {
    let mut builder = MysqlSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;

    if let Some(tgid) = args.tgid {
        open.rodata().tool_config.tgid = tgid;
    }

    if let Some(port) = args.mysql_port {
        open.rodata().tool_config.target_port = port;
    }

    let mut load = open.load()?;
    load.attach()?;

    let buffer = libbpf_rs::PerfBufferBuilder::new(load.maps_mut().events())
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = mysql_bss_types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");

            let packet_length =
                event.buf[0] as u32 | ((event.buf[1] as u32) << 8) | ((event.buf[2] as u32) << 16);
            let sequence_id = event.buf[3];

            let payload = if event.len > 4 {
                let end_idx = std::cmp::min(event.len as usize, event.buf.len());
                &event.buf[4..end_idx]
            } else {
                &[]
            };

            let direction = match event.direction {
                mysql_bss_types::packet_direction::CLIENT_TO_SERVER => "CLIENT->SERVER",
                mysql_bss_types::packet_direction::SERVER_TO_CLIENT => "SERVER->CLIENT",
            };

            // Parse command or response using mysql-protocol-parser
            let (parsed_command, parsed_response, handshake_info) = if event.direction
                == mysql_bss_types::packet_direction::CLIENT_TO_SERVER
                && !payload.is_empty()
            {
                // Check if this is a handshake response (sequence_id == 1 and large packet)
                if sequence_id == 1 && packet_length > 32 {
                    // This is likely a handshake response packet
                    (
                        None,
                        None,
                        Some("Handshake Response (Client Authentication)".to_string()),
                    )
                } else {
                    // Parse client command
                    match mysql_protocol_parser::command::parse_command(payload) {
                        Ok(cmd) => (Some(cmd), None, None),
                        Err(_) => (None, None, None),
                    }
                }
            } else if event.direction == mysql_bss_types::packet_direction::SERVER_TO_CLIENT
                && !payload.is_empty()
            {
                // Check for initial handshake packet (sequence_id == 0)
                if sequence_id == 0 && payload.len() > 4 {
                    // Try to parse as initial handshake
                    if let Some(version) = parse_handshake_v10(payload) {
                        (
                            None,
                            None,
                            Some(format!("Initial Handshake (Server Version: {version})")),
                        )
                    } else {
                        (None, None, None)
                    }
                } else if sequence_id == 2 && packet_length == 2 {
                    // Auth switch request or more auth data
                    (None, None, Some("Auth Switch Request".to_string()))
                } else {
                    // For server responses, we need the full packet with header
                    // The payload we have doesn't include the header, so we'll reconstruct it
                    let mut full_packet = Vec::with_capacity(4 + payload.len());
                    full_packet.push((packet_length & 0xFF) as u8);
                    full_packet.push(((packet_length >> 8) & 0xFF) as u8);
                    full_packet.push(((packet_length >> 16) & 0xFF) as u8);
                    full_packet.push(sequence_id);
                    full_packet.extend_from_slice(payload);

                    // Parse server response
                    match mysql_protocol_parser::resultset::parse_query_response(&full_packet) {
                        Ok(response) => (None, Some(response), None),
                        Err(_) => (None, None, None),
                    }
                }
            } else {
                (None, None, None)
            };

            let mysql_event = crate::ResultEvent::Mysql {
                tgid: event.tgid,
                pid: event.pid as u32,
                uid: event.uid,
                fd: event.fd,
                direction: direction.to_string(),
                packet_length,
                sequence_id,
                parsed_command,
                parsed_response,
                handshake_info,
                data: payload.to_vec(),
            };

            tx.send(crate::Event::Result(mysql_event)).unwrap();
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

fn parse_handshake_v10(payload: &[u8]) -> Option<String> {
    // Protocol version (1 byte) - should be 0x0a for v10
    if payload.is_empty() || payload[0] != 0x0a {
        return None;
    }

    // Find null terminator for server version string
    let version_end = payload[1..].iter().position(|&b| b == 0)?;
    let version = std::str::from_utf8(&payload[1..1 + version_end]).ok()?;

    Some(version.to_string())
}
