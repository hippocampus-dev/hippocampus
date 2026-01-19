use clap::Parser;

mod bpf;
mod server;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(long, default_value = "127.0.0.1:8080")]
    address: String,

    #[clap(short, long, default_value = "/var/log/containers")]
    directory: std::path::PathBuf,

    #[clap(
        short = 'f',
        long,
        default_value = "/var/log/fluentd-containers.log.pos"
    )]
    pos_file: std::path::PathBuf,

    #[clap(short = 's', long, default_value_t = 1)]
    delayed_seconds: u64,
}

struct PosCacheEntry {
    mtime: std::time::SystemTime,
    entries: std::collections::HashMap<String, (u64, u64)>,
}

type PosCache = std::sync::Arc<tokio::sync::RwLock<PosCacheEntry>>;

fn parse_pos_file(path: &std::path::Path) -> std::collections::HashMap<String, (u64, u64)> {
    let mut pos = std::collections::HashMap::new();
    if let Ok(file) = std::fs::read_to_string(path) {
        for line in file.lines() {
            let mut parts = line.split_whitespace();
            if let (Some(path), Some(offset), Some(inode)) =
                (parts.next(), parts.next(), parts.next())
            {
                let resolved_path = std::fs::canonicalize(path)
                    .map(|p| p.to_string_lossy().to_string())
                    .unwrap_or_else(|_| path.to_string());
                pos.insert(
                    resolved_path,
                    (
                        u64::from_str_radix(offset, 16).unwrap_or_default(),
                        u64::from_str_radix(inode, 16).unwrap_or_default(),
                    ),
                );
            }
        }
    }
    pos
}

fn start(
    directory: std::path::PathBuf,
    tx: tokio::sync::mpsc::UnboundedSender<String>,
) -> std::sync::Arc<std::sync::atomic::AtomicBool> {
    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));

    let cloned_stop = std::sync::Arc::clone(&stop);
    std::thread::spawn(move || {
        bpf::watch(&directory, tx, cloned_stop).unwrap();
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

    let (tx, mut rx): (
        tokio::sync::mpsc::UnboundedSender<std::path::PathBuf>,
        tokio::sync::mpsc::UnboundedReceiver<std::path::PathBuf>,
    ) = tokio::sync::mpsc::unbounded_channel();

    tx.send(args.directory)?;

    let (unlink_tx, mut unlink_rx): (
        tokio::sync::mpsc::UnboundedSender<String>,
        tokio::sync::mpsc::UnboundedReceiver<String>,
    ) = tokio::sync::mpsc::unbounded_channel();

    let pos_cache: PosCache = std::sync::Arc::new(tokio::sync::RwLock::new(PosCacheEntry {
        mtime: std::time::SystemTime::UNIX_EPOCH,
        entries: std::collections::HashMap::new(),
    }));

    let cloned_unlink_tx = unlink_tx.clone();
    let mut before_stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    handles.push(tokio::spawn(async move {
        while let Some(directory) = rx.recv().await {
            let cloned_unlink_tx = cloned_unlink_tx.clone();
            let after_stop = start(directory, cloned_unlink_tx);
            before_stop.store(true, std::sync::atomic::Ordering::Relaxed);
            before_stop = after_stop;
        }
    }));

    handles.push(tokio::spawn(async move {
        let meter = opentelemetry::global::meter("fluentd-delayed-unlink");
        let counter = meter.u64_counter("delayed_unlink_total").init();

        while let Some(pathname) = unlink_rx.recv().await {
            let current_mtime = std::fs::metadata(&args.pos_file)
                .and_then(|m| m.modified())
                .unwrap_or(std::time::SystemTime::UNIX_EPOCH);

            let needs_refresh = pos_cache.read().await.mtime != current_mtime;
            if needs_refresh {
                let new_pos = parse_pos_file(&args.pos_file);
                let mut cache = pos_cache.write().await;
                cache.mtime = current_mtime;
                cache.entries = new_pos;
            }

            if let Some(offset) = pos_cache
                .read()
                .await
                .entries
                .get(&pathname)
                .map(|(offset, _)| *offset)
            {
                let path = std::path::Path::new(&pathname);
                if let Ok(metadata) = path.metadata()
                    && metadata.len() != offset
                {
                    counter.add(&opentelemetry::Context::current(), 1, &[]);
                    let unlink_tx = unlink_tx.clone();
                    tokio::spawn(async move {
                        tokio::time::sleep(std::time::Duration::from_secs(
                            args.delayed_seconds,
                        ))
                        .await;
                        if let Err(e) = unlink_tx.send(pathname.clone()) {
                            eprintln!("Failed to re-queue {}: {}", pathname, e);
                        }
                    });
                    continue;
                }
            }

            println!("Removing {}", pathname);

            let result = std::fs::remove_file(&pathname).or_else(|e| {
                if e.kind() == std::io::ErrorKind::IsADirectory {
                    std::fs::remove_dir(&pathname)
                } else {
                    Err(e)
                }
            });
            if let Err(e) = result {
                if e.kind() == std::io::ErrorKind::DirectoryNotEmpty {
                    if let Ok(entries) = std::fs::read_dir(&pathname) {
                        for entry in entries.flatten() {
                            let entry_path = entry.path().to_string_lossy().to_string();
                            if let Err(e) = unlink_tx.send(entry_path.clone()) {
                                eprintln!("Failed to queue {}: {}", entry_path, e);
                            }
                        }
                    }
                    let unlink_tx = unlink_tx.clone();
                    tokio::spawn(async move {
                        tokio::time::sleep(std::time::Duration::from_secs(
                            args.delayed_seconds,
                        ))
                        .await;
                        if let Err(e) = unlink_tx.send(pathname.clone()) {
                            eprintln!("Failed to re-queue {}: {}", pathname, e);
                        }
                    });
                } else if e.kind() != std::io::ErrorKind::NotFound {
                    eprintln!("Failed to remove {}: {}", pathname, e);
                }
            }
        }
    }));

    server.await?;

    for handle in handles {
        handle.abort();
    }

    Ok(())
}
