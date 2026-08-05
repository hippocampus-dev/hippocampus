mod skel {
    include!("bpf/skel.rs");
}

use libbpf_rs::skel::{OpenSkel, Skel, SkelBuilder};
use std::mem::MaybeUninit;

const ADDR_LEN: usize = 32;

unsafe impl plain::Plain for skel::types::event {}

pub fn watch(
    map: crate::IPMap,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let builder = skel::ConnectSkelBuilder::default();
    let mut uninit = MaybeUninit::uninit();
    let open = builder.open(&mut uninit)?;

    let v4_keys = map.ipv4.keys();
    let mut v4_keys_array: [u32; ADDR_LEN] = [0; ADDR_LEN];
    let v4_keys_len = v4_keys.len();
    if v4_keys_len > ADDR_LEN {
        return Err("Too many IPv4 keys".into());
    }
    for (i, key) in v4_keys.enumerate() {
        v4_keys_array[i] = *key;
    }
    open.maps.rodata_data.tool_config.daddr_v4 = v4_keys_array;
    open.maps.rodata_data.tool_config.daddr_v4_len = v4_keys_len as u32;

    let v6_keys = map.ipv6.keys();
    let mut v6_keys_array: [[u8; 16]; ADDR_LEN] = [[0; 16]; ADDR_LEN];
    let v6_keys_len = v6_keys.len();
    if v6_keys_len > ADDR_LEN {
        return Err("Too many IPv6 keys".into());
    }
    for (i, key) in v6_keys.enumerate() {
        v6_keys_array[i] = (*key).to_be_bytes();
    }
    open.maps.rodata_data.tool_config.daddr_v6 = v6_keys_array;
    open.maps.rodata_data.tool_config.daddr_v6_len = v6_keys_len as u32;

    let mut load = open.load()?;
    load.attach()?;

    let meter = opentelemetry::global::meter("connectracer");
    let counter = meter.u64_counter("connect_total").init();

    let buffer = libbpf_rs::PerfBufferBuilder::new(&load.maps.events)
        .sample_cb(move |_cpu: i32, data: &[u8]| {
            let mut event = skel::types::event::default();
            plain::copy_from_bytes(&mut event, data).expect("Data buffer was too short");

            let protocol = unsafe { event.protocol.assume_init() };
            let host = match protocol {
                skel::types::protocol::ipv4 => map.ipv4.get(&event.daddr_v4),
                skel::types::protocol::ipv6 => map.ipv6.get(&u128::from_be_bytes(event.daddr_v6)),
            };

            if let Some(host) = host {
                let command = if let Ok(s) = std::str::from_utf8(&event.comm) {
                    s.trim_end_matches(char::from(0))
                } else {
                    ""
                };

                let mut attributes = vec![
                    opentelemetry::KeyValue::new("host", host.clone()),
                    opentelemetry::KeyValue::new("port", event.dport.to_string()),
                    opentelemetry::KeyValue::new("command", command.to_string()),
                ];

                if let Some(metadata) = crate::metadata::kubernetes::from_pid(event.pid) {
                    let mut m = metadata.into();
                    attributes.append(&mut m);
                }

                counter.add(&opentelemetry::Context::current(), 1, &attributes);
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
