use clap::Parser;
use termion::input::TermRead;
use termion::raw::IntoRawMode;

const HTTP_PREFIX: &str = "[HTTP] ";
const MYSQL_PREFIX: &str = "[MySQL] ";

fn main() -> Result<(), error::Error> {
    let args: insight::Args = insight::Args::parse();

    let (event_tx, event_rx): (
        std::sync::mpsc::Sender<insight::Event>,
        std::sync::mpsc::Receiver<insight::Event>,
    ) = std::sync::mpsc::channel();

    let stop = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let shutdown = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let mut handles = Vec::new();
    let http_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_shutdown = std::sync::Arc::clone(&shutdown);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::http::watch(http_args, cloned_stop, cloned_shutdown, cloned_event_tx).unwrap();
    }));
    let https_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_shutdown = std::sync::Arc::clone(&shutdown);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::https::watch(https_args, cloned_stop, cloned_shutdown, cloned_event_tx).unwrap();
    }));
    let mysql_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_shutdown = std::sync::Arc::clone(&shutdown);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::mysql::watch(mysql_args, cloned_stop, cloned_shutdown, cloned_event_tx).unwrap();
    }));
    let vfs_args = args.clone();
    let cloned_stop = std::sync::Arc::clone(&stop);
    let cloned_shutdown = std::sync::Arc::clone(&shutdown);
    let cloned_event_tx = event_tx.clone();
    handles.push(std::thread::spawn(|| {
        insight::vfs::watch(vfs_args, cloned_stop, cloned_shutdown, cloned_event_tx).unwrap();
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
                let mut buffer = String::new();
                if <std::io::BufReader<std::fs::File> as std::io::BufRead>::read_line(
                    &mut reader,
                    &mut buffer,
                )
                .is_ok()
                {
                    cloned_event_tx
                        .send(insight::Event::Result(insight::ResultEvent::TracePipe(
                            buffer.clone(),
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
                        // Split HTTP payload by lines and add [HTTP] prefix to the first line only
                        let lines: Vec<&str> = payload.lines().collect();
                        if !lines.is_empty() {
                            // First line with [HTTP] prefix
                            let first_line = tui::text::Spans::from(vec![
                                tui::text::Span::styled(
                                    HTTP_PREFIX,
                                    tui::style::Style::default()
                                        .fg(tui::style::Color::Green)
                                        .add_modifier(tui::style::Modifier::BOLD),
                                ),
                                tui::text::Span::raw(lines[0].to_string()),
                            ]);
                            state.l7.extend(vec![first_line]);

                            // Remaining lines with HTTP-specific indentation
                            let http_indent = " ".repeat(HTTP_PREFIX.len());
                            for line in lines.iter().skip(1) {
                                let continuation_line = tui::text::Spans::from(vec![
                                    tui::text::Span::raw(http_indent.clone()),
                                    tui::text::Span::raw(line.to_string()),
                                ]);
                                state.l7.extend(vec![continuation_line]);
                            }
                        }
                    }
                    insight::ResultEvent::TracePipe(payload) => state
                        .trace_pipe
                        .extend(tui::text::Text::raw(payload.clone())),
                    insight::ResultEvent::Mysql {
                        tgid: _,
                        pid,
                        uid: _,
                        fd,
                        direction,
                        packet_length,
                        sequence_id,
                        parsed_command,
                        parsed_response,
                        handshake_info,
                        data,
                    } => {
                        let mut spans = Vec::new();

                        spans.push(tui::text::Span::styled(
                            MYSQL_PREFIX,
                            tui::style::Style::default()
                                .fg(tui::style::Color::Cyan)
                                .add_modifier(tui::style::Modifier::BOLD),
                        ));

                        let direction_color = if direction == "CLIENT->SERVER" {
                            tui::style::Color::Yellow
                        } else {
                            tui::style::Color::Magenta
                        };
                        spans.push(tui::text::Span::styled(
                            format!("{} ", direction),
                            tui::style::Style::default().fg(direction_color),
                        ));

                        spans.push(tui::text::Span::raw(format!(
                            "PID:{} FD:{} Len:{} Seq:{} ",
                            pid, fd, packet_length, sequence_id
                        )));

                        if let Some(ref info) = handshake_info {
                            spans.push(tui::text::Span::styled(
                                info.clone(),
                                tui::style::Style::default().fg(tui::style::Color::Gray),
                            ));
                        } else if let Some(ref cmd) = parsed_command {
                            match cmd {
                                mysql_protocol_parser::Command::Query(q) => {
                                    spans.push(tui::text::Span::styled(
                                        "Query: ",
                                        tui::style::Style::default()
                                            .fg(tui::style::Color::White)
                                            .add_modifier(tui::style::Modifier::BOLD),
                                    ));
                                    spans.push(tui::text::Span::styled(
                                        q.query.clone(),
                                        tui::style::Style::default().fg(tui::style::Color::White),
                                    ));
                                }
                                mysql_protocol_parser::Command::InitDb(db) => {
                                    spans.push(tui::text::Span::raw(format!(
                                        "Cmd:COM_INIT_DB Database: {}",
                                        db.database
                                    )));
                                }
                                mysql_protocol_parser::Command::Ping => {
                                    spans.push(tui::text::Span::raw("Cmd:COM_PING"));
                                }
                                mysql_protocol_parser::Command::Quit => {
                                    spans.push(tui::text::Span::raw("Cmd:COM_QUIT"));
                                }
                                mysql_protocol_parser::Command::Statistics => {
                                    spans.push(tui::text::Span::raw("Cmd:COM_STATISTICS"));
                                }
                                _ => {
                                    spans.push(tui::text::Span::raw(format!("Cmd:{:?}", cmd)));
                                }
                            }
                        } else if let Some(ref response) = parsed_response {
                            match response {
                                mysql_protocol_parser::QueryResponse::Ok(ok) => {
                                    spans.push(tui::text::Span::styled(
                                        format!(
                                            "Response: OK (affected_rows={}, last_insert_id={}, status={:#x}, warnings={})",
                                            ok.affected_rows, ok.last_insert_id, ok.status_flags, ok.warnings
                                        ),
                                        tui::style::Style::default().fg(tui::style::Color::Green),
                                    ));
                                }
                                mysql_protocol_parser::QueryResponse::Error(err) => {
                                    spans.push(tui::text::Span::styled(
                                        format!(
                                            "Response: ERROR {} ({}): {}",
                                            err.error_code,
                                            err.sql_state.as_deref().unwrap_or("HY000"),
                                            err.error_message
                                        ),
                                        tui::style::Style::default().fg(tui::style::Color::Red),
                                    ));
                                }
                                mysql_protocol_parser::QueryResponse::ResultSet(rs) => {
                                    spans.push(tui::text::Span::styled(
                                        format!(
                                            "Response: ResultSet (columns={}) ",
                                            rs.columns.len()
                                        ),
                                        tui::style::Style::default().fg(tui::style::Color::Blue),
                                    ));
                                    if !rs.columns.is_empty() {
                                        let column_names: Vec<_> =
                                            rs.columns.iter().map(|c| &c.name).collect();
                                        spans.push(tui::text::Span::raw(format!(
                                            "Columns: {:?} ",
                                            column_names
                                        )));
                                    }
                                    if !rs.rows.is_empty() {
                                        spans.push(tui::text::Span::raw(format!(
                                            "Rows: {}",
                                            rs.rows.len()
                                        )));
                                        let styled_spans = tui::text::Spans::from(spans.clone());
                                        state.l7.extend(vec![styled_spans]);

                                        for (i, row) in rs.rows.iter().take(10).enumerate() {
                                            let mut row_str = Vec::new();
                                            for value in &row.values {
                                                if let Some(bytes) = value {
                                                    if let Ok(s) = std::str::from_utf8(bytes) {
                                                        row_str.push(s.to_string());
                                                    } else {
                                                        row_str
                                                            .push(format!("(hex:{:02x?})", bytes));
                                                    }
                                                } else {
                                                    row_str.push("NULL".to_string());
                                                }
                                            }
                                            let row_indent = " ".repeat(MYSQL_PREFIX.len() + 2);
                                            let row_spans = tui::text::Spans::from(vec![
                                                tui::text::Span::raw(row_indent.clone()),
                                                tui::text::Span::raw("Row "),
                                                tui::text::Span::styled(
                                                    format!("{}", i),
                                                    tui::style::Style::default()
                                                        .fg(tui::style::Color::Yellow),
                                                ),
                                                tui::text::Span::raw(format!(
                                                    ": [{}]",
                                                    row_str.join(", ")
                                                )),
                                            ]);
                                            state.l7.extend(vec![row_spans]);
                                        }
                                        if rs.rows.len() > 10 {
                                            let more_indent = " ".repeat(MYSQL_PREFIX.len() + 2);
                                            let more_spans = tui::text::Spans::from(vec![
                                                tui::text::Span::raw(more_indent),
                                                tui::text::Span::raw(format!(
                                                    "... and {} more rows",
                                                    rs.rows.len() - 10
                                                )),
                                            ]);
                                            state.l7.extend(vec![more_spans]);
                                        }
                                        continue;
                                    }
                                }
                                mysql_protocol_parser::QueryResponse::StmtPrepareOk(stmt) => {
                                    spans.push(tui::text::Span::raw(format!(
                                        "Response: StmtPrepareOk (statement_id={}, columns={}, params={})",
                                        stmt.statement_id, stmt.num_columns, stmt.num_params
                                    )));
                                }
                            }
                        } else if data.len() > 0
                            && parsed_command.is_none()
                            && parsed_response.is_none()
                            && handshake_info.is_none()
                        {
                            spans.push(tui::text::Span::raw(format!(
                                "Data: {:02x?}",
                                &data.iter().take(20).collect::<Vec<_>>()
                            )));
                        }

                        let styled_spans = tui::text::Spans::from(spans);
                        state.l7.extend(vec![styled_spans]);
                    }
                    insight::ResultEvent::Vfs {
                        pid: _,
                        tgid,
                        uid,
                        timestamp_ns: _,
                        size,
                        ret,
                        op,
                        comm,
                        filename,
                    } => {
                        state
                            .vfs_stats_table
                            .stats
                            .entry(filename.clone())
                            .or_insert_with(|| {
                                insight::ui::vfs::VfsFileStats::new(filename.clone())
                            })
                            .update(&op, size);

                        let status = if ret < 0 {
                            insight::errno_to_name(ret)
                        } else {
                            "OK"
                        };
                        let event_line =
                            tui::text::Spans::from(vec![tui::text::Span::raw(format!(
                                "{:>6} {:>6} {:<16} {:<6} {:>12} {:<12} {}",
                                tgid,
                                uid,
                                comm,
                                op.label(),
                                size,
                                status,
                                filename
                            ))]);
                        state.vfs_events.extend(vec![event_line]);
                    }
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
    shutdown.store(true, std::sync::atomic::Ordering::Relaxed);
    for handle in handles {
        let _ = handle.join();
    }
    Ok(())
}
