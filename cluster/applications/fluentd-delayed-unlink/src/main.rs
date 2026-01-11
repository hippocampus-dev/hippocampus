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

    let cloned_unlink_tx = unlink_tx.clone();
    let mut before_stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    handles.push(tokio::spawn(async move {
        loop {
            if let Ok(Some(directory)) =
                tokio::time::timeout(std::time::Duration::from_secs(1), rx.recv()).await
            {
                let cloned_unlink_tx = cloned_unlink_tx.clone();
                let after_stop = start(directory, cloned_unlink_tx);
                before_stop.store(true, std::sync::atomic::Ordering::Relaxed);
                before_stop = after_stop;
            }
        }
    }));

    handles.push(tokio::spawn(async move {
        let meter = opentelemetry::global::meter("fluentd-delayed-unlink");
        let counter = meter.u64_counter("delayed_unlink_total").init();

        loop {
            if let Ok(Some(pathname)) =
                tokio::time::timeout(std::time::Duration::from_secs(1), unlink_rx.recv()).await
            {
                println!("Removing {}", pathname);

                if let Ok(file) = std::fs::read_to_string(&args.pos_file) {
                    let mut pos = std::collections::HashMap::new();
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

                    if let Some((offset, _inode)) = pos.get(&pathname) {
                        let path = std::path::Path::new(&pathname);
                        if let Ok(metadata) = path.metadata() {
                            if metadata.len() != *offset {
                                counter.add(&opentelemetry::Context::current(), 1, &[]);
                                let unlink_tx = unlink_tx.clone();
                                tokio::spawn(async move {
                                    tokio::time::sleep(std::time::Duration::from_secs(
                                        args.delayed_seconds,
                                    ))
                                    .await;
                                    if let Err(e) = unlink_tx.send(pathname.clone()) {
                                        eprintln!("Failed to remove {}: {}", pathname, e);
                                    }
                                });
                                continue;
                            }
                        }
                    }
                }

                if let Err(e) = std::fs::remove_file(&pathname) {
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
