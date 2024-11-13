use std::str::FromStr;

use clap::Parser;
use trust_dns_client::client::ClientHandle;

mod bpf;
mod metadata;
mod server;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(long, default_value = "127.0.0.1:8080")]
    address: String,

    #[clap(long, default_value = "1.1.1.1:53")]
    nameserver: String,

    #[clap(short, long)]
    hosts: Vec<String>,

    #[clap(short, long)]
    addresses: Vec<String>,
}

#[derive(Clone, Debug)]
pub struct IPMap {
    ipv4: std::collections::HashMap<u32, String>,
    ipv6: std::collections::HashMap<u128, String>,
}

impl IPMap {
    fn new() -> Self {
        Self {
            ipv4: std::collections::HashMap::new(),
            ipv6: std::collections::HashMap::new(),
        }
    }
}

struct IPCache {
    ipv4: std::collections::HashMap<String, Vec<u32>>,
    ipv6: std::collections::HashMap<String, Vec<u128>>,
}

impl IPCache {
    fn new() -> Self {
        Self {
            ipv4: std::collections::HashMap::new(),
            ipv6: std::collections::HashMap::new(),
        }
    }
}

fn start(hm: IPMap) -> std::sync::Arc<std::sync::atomic::AtomicBool> {
    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));

    let cloned_stop = std::sync::Arc::clone(&stop);
    std::thread::spawn(move || {
        bpf::watch(hm, cloned_stop).unwrap();
    });

    std::sync::Arc::clone(&stop)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let args: Args = Args::parse();

    let address = args.address.parse::<std::net::SocketAddr>()?;
    let exporter = opentelemetry_prometheus::exporter(
        opentelemetry::sdk::metrics::controllers::basic(
            opentelemetry::sdk::metrics::processors::factory(
                opentelemetry::sdk::metrics::selectors::simple::inexpensive(),
                opentelemetry::sdk::export::metrics::aggregation::cumulative_temporality_selector(),
            )
            .with_memory(true),
        )
        .build(),
    )
    .init();
    let signal = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    let server = server::serve(address, exporter, signal);

    let mut handles = Vec::new();

    let ns = args.nameserver.parse::<std::net::SocketAddr>()?;
    let conn = trust_dns_client::udp::UdpClientStream::<tokio::net::UdpSocket>::with_timeout(
        ns,
        std::time::Duration::from_secs(5),
    );
    let (client, bg) = trust_dns_client::client::AsyncClient::connect(conn).await?;
    handles.push(tokio::spawn(bg));

    let (tx, mut rx): (
        tokio::sync::mpsc::UnboundedSender<IPMap>,
        tokio::sync::mpsc::UnboundedReceiver<IPMap>,
    ) = tokio::sync::mpsc::unbounded_channel();
    let hm = std::sync::Arc::new(futures::lock::Mutex::new(IPMap::new()));

    for host in args.hosts {
        for record_type in [
            trust_dns_client::rr::RecordType::A,
            trust_dns_client::rr::RecordType::AAAA,
        ] {
            let mut cloned_client = client.clone();
            let cloned_hm = std::sync::Arc::clone(&hm);
            let cloned_tx = tx.clone();
            let host = host.clone();
            handles.push(tokio::spawn(async move {
                let name = trust_dns_client::rr::Name::from_str(&host).unwrap();
                let mut cache = IPCache::new();
                loop {
                    let response: trust_dns_client::op::DnsResponse = cloned_client
                        .query(
                            name.clone(),
                            trust_dns_client::rr::DNSClass::IN,
                            record_type,
                        )
                        .await
                        .unwrap();
                    let answers: &[trust_dns_client::rr::Record] = response.answers();
                    let mut max_ttl = 0;

                    match record_type {
                        trust_dns_client::proto::rr::RecordType::A => {
                            let mut new = Vec::new();
                            for record in answers {
                                if record.ttl() > max_ttl {
                                    max_ttl = record.ttl();
                                }
                                if let Some(trust_dns_client::proto::rr::RData::A(ref ip)) =
                                    record.data()
                                {
                                    new.push(u32::swap_bytes((*ip).into()))
                                }
                            }
                            new.sort();

                            let default = Vec::new();
                            let old = cache.ipv4.get(&host).unwrap_or(&default);
                            if old != &new {
                                let mut hm = cloned_hm.lock().await;
                                if let trust_dns_client::proto::rr::RecordType::A = record_type {
                                    for ip in old {
                                        hm.ipv4.remove(ip);
                                    }
                                    for ip in &new {
                                        hm.ipv4.insert(*ip, host.clone());
                                    }
                                }
                                cloned_tx.send(hm.clone()).unwrap();

                                cache.ipv4.insert(host.clone(), new);
                            }
                        }
                        trust_dns_client::proto::rr::RecordType::AAAA => {
                            let mut new = Vec::new();
                            for record in answers {
                                if record.ttl() > max_ttl {
                                    max_ttl = record.ttl();
                                }
                                if let Some(trust_dns_client::proto::rr::RData::AAAA(ref ip)) =
                                    record.data()
                                {
                                    new.push((*ip).into())
                                }
                            }
                            new.sort();

                            let default = Vec::new();
                            let old = cache.ipv6.get(&host).unwrap_or(&default);
                            if old != &new {
                                let mut hm = cloned_hm.lock().await;
                                if let trust_dns_client::proto::rr::RecordType::AAAA = record_type {
                                    for ip in old {
                                        hm.ipv6.remove(ip);
                                    }
                                    for ip in &new {
                                        hm.ipv6.insert(*ip, host.clone());
                                    }
                                }
                                cloned_tx.send(hm.clone()).unwrap();

                                cache.ipv6.insert(host.clone(), new);
                            }
                        }
                        _ => {
                            continue;
                        }
                    }

                    if max_ttl > 60 {
                        tokio::time::sleep(std::time::Duration::from_secs(max_ttl as u64)).await;
                    } else {
                        tokio::time::sleep(std::time::Duration::from_secs(60)).await;
                    }
                }
            }));
        }
    }

    let mut before_stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    handles.push(tokio::spawn(async move {
        loop {
            if let Ok(value) =
                tokio::time::timeout(std::time::Duration::from_secs(1), rx.recv()).await
            {
                if let Some(hm) = value {
                    let after_stop = start(hm);
                    before_stop.store(true, std::sync::atomic::Ordering::Relaxed);
                    before_stop = after_stop;
                }
            }
        }
    }));

    {
        let cloned_hm = std::sync::Arc::clone(&hm);
        let mut hm = cloned_hm.lock().await;
        for address in args.addresses {
            if let Ok(ip) = address.parse::<std::net::IpAddr>() {
                match ip {
                    std::net::IpAddr::V4(ip) => {
                        hm.ipv4.insert(u32::swap_bytes(ip.into()), address.clone());
                    }
                    std::net::IpAddr::V6(ip) => {
                        hm.ipv6.insert(ip.into(), address.clone());
                    }
                }
            }
        }
        tx.send(hm.clone()).unwrap();
    }

    server.await?;

    for handle in handles {
        handle.abort();
    }

    Ok(())
}
