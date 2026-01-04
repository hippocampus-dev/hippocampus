use clap::Parser;
fn main() -> Result<(), error::Error> {
    let args: insight::Args = insight::Args::parse();

    println!(
        "{:>10} {:>6} {:>6} {:>6} {:<16} {:<6} {:>12} {:<12} FILE",
        "TIME(s)", "TGID", "PID", "UID", "COMM", "OP", "BYTES", "STATUS"
    );

    let (tx, rx) = std::sync::mpsc::channel();
    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let shutdown = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));

    let stop_clone = stop.clone();
    let shutdown_clone = shutdown.clone();
    let handle = std::thread::spawn(move || {
        insight::vfs::watch(args, stop_clone, shutdown_clone, tx)
    });

    for event in rx {
        if let insight::Event::Result(insight::ResultEvent::Vfs {
            pid,
            tgid,
            uid,
            timestamp_ns,
            size,
            ret,
            op,
            comm,
            filename,
        }) = event
        {
            let timestamp_seconds = timestamp_ns as f64 / 1_000_000_000.0;
            let status = if ret < 0 {
                insight::errno_to_name(ret)
            } else {
                "OK"
            };
            println!(
                "{:>10.6} {:>6} {:>6} {:>6} {:<16} {:<6} {:>12} {:<12} {}",
                timestamp_seconds,
                tgid,
                pid,
                uid,
                comm,
                op.label(),
                size,
                status,
                filename
            );
        }
    }

    handle.join().unwrap()?;

    Ok(())
}
