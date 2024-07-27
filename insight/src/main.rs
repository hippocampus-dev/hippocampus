use clap::Parser;
use termion::input::TermRead;
use termion::raw::IntoRawMode;

fn main() -> Result<(), error::Error> {
    let args: insight::Args = insight::Args::parse();

    let (event_tx, event_rx): (
        std::sync::mpsc::Sender<insight::Event>,
        std::sync::mpsc::Receiver<insight::Event>,
    ) = std::sync::mpsc::channel();

    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let mut handles = Vec::new();
    let http_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::http::watch(http_args, cloned_stop, cloned_event_tx).unwrap();
    }));
    let https_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::https::watch(https_args, cloned_stop, cloned_event_tx).unwrap();
    }));
    let stdin = std::io::stdin();
    let stdout = termion::screen::AlternateScreen::from(std::io::stdout()).into_raw_mode()?;
    let backend = tui::backend::TermionBackend::new(stdout);
    let mut terminal = tui::Terminal::new(backend)?;

    if cfg!(debug_assertions) {
        let cloned_event_tx = event_tx.clone();
        std::thread::spawn(move || {
            let mut reader = std::io::BufReader::new(
                std::fs::File::open("/sys/kernel/debug/tracing/trace_pipe").unwrap(),
            );
            loop {
                let mut buf = String::new();
                if <std::io::BufReader<std::fs::File> as std::io::BufRead>::read_line(
                    &mut reader,
                    &mut buf,
                )
                .is_ok()
                {
                    cloned_event_tx
                        .send(insight::Event::Result(insight::ResultEvent::TracePipe(
                            buf.clone(),
                        )))
                        .unwrap();
                }
            }
        });
    }

    std::thread::spawn(move || {
        for event in stdin.events().flatten() {
            event_tx.send(insight::Event::Input(event)).unwrap();
        }
    });

    let mut state = insight::ui::State::new(args);
    loop {
        terminal.draw(|f| insight::ui::draw(f, &mut state))?;
        if let Ok(event) = event_rx.try_recv() {
            match event {
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Char(
                    '\t',
                ))) => {
                    state.next_tab();
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::BackTab)) => {
                    state.previous_tab();
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Ctrl(
                    'c',
                ))) => {
                    break;
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Char(
                    'G',
                ))) => {
                    state.bottom();
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Char(
                    's',
                ))) => {
                    if stop.load(std::sync::atomic::Ordering::Relaxed) {
                        stop.store(false, std::sync::atomic::Ordering::Relaxed);
                    } else {
                        stop.store(true, std::sync::atomic::Ordering::Relaxed);
                    }
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Up)) => {
                    state.on_up();
                }
                insight::Event::Input(termion::event::Event::Key(termion::event::Key::Down)) => {
                    state.on_down();
                }
                insight::Event::Result(result) => match result {
                    insight::ResultEvent::L7(payload) => {
                        state.l7.extend(tui::text::Text::raw(payload.clone()))
                    }
                    insight::ResultEvent::TracePipe(payload) => state
                        .trace_pipe
                        .extend(tui::text::Text::raw(payload.clone())),
                },
                insight::Event::Histogram(histogram) => match histogram {
                    insight::HistogramEvent::CPUUsage(payload) => state.cpu_usage = payload,
                },
                insight::Event::Gauge(gauge) => match gauge {
                    insight::GaugeEvent::CPUUtilization(payload) => {
                        if state.cpu_utilization.len() == insight::ui::CHART_HISTORY_LIMIT {
                            let _ = state.cpu_utilization.pop_front();
                        }
                        state.cpu_utilization.push_back(payload)
                    }
                },
                _ => {}
            }
        } else {
            std::thread::sleep(std::time::Duration::from_millis(100));
        }
    }
    stop.store(true, std::sync::atomic::Ordering::Relaxed);
    for handle in handles {
        let _ = handle.join();
    }

    Ok(())
}
