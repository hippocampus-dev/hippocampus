use clap::Parser;

fn main() {
    let args: insight::Args = insight::Args::parse();

    let (event_tx, event_rx): (
        std::sync::mpsc::Sender<insight::Event>,
        std::sync::mpsc::Receiver<insight::Event>,
    ) = std::sync::mpsc::channel();

    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let http_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_event_tx = event_tx.clone();
    std::thread::spawn(|| {
        insight::http::watch(http_args, cloned_stop, cloned_event_tx).unwrap();
    });
    let https_args = args;
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_event_tx = event_tx;
    std::thread::spawn(|| {
        insight::https::watch(https_args, cloned_stop, cloned_event_tx).unwrap();
    });

    loop {
        if let Ok(event) = event_rx.try_recv() {
            if let insight::Event::Result(result) = event {
                match result {
                    insight::ResultEvent::L7(payload) => {
                        println!("{}", payload)
                    }
                    insight::ResultEvent::TracePipe(payload) => {
                        println!("{}", payload)
                    }
                }
            }
        } else {
            std::thread::sleep(std::time::Duration::from_millis(100));
        }
    }
}
