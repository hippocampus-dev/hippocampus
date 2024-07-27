use std::io::Write;

use termion::input::TermRead;
use termion::raw::IntoRawMode;

#[derive(Clone, Debug)]
enum Mode {
    Normal,
    Detail,
}

#[derive(Clone, Debug)]
struct LastState {
    state: Option<State>,
    timestamp: Option<std::time::Instant>,
}

impl LastState {
    fn new() -> Self {
        Self {
            state: None,
            timestamp: None,
        }
    }

    fn get(&self) -> Option<State> {
        self.state.clone()
    }

    fn set(&mut self, state: State) {
        self.state = Some(state);
        self.timestamp = Some(std::time::Instant::now())
    }

    fn elapsed(&self) -> Option<std::time::Duration> {
        self.timestamp.map(|timestamp| timestamp.elapsed())
    }

    fn reset(&mut self) {
        self.state = None;
        self.timestamp = None;
    }
}

#[derive(Clone, Debug)]
struct State {
    mode: Mode,
    page: usize,
    page_size: usize,
    cursor: usize,
    query: String,
    results: Vec<hippocampus_core::searcher::SearchResult>,
}

impl State {
    fn new() -> Self {
        Self {
            mode: Mode::Normal,
            page: 1,
            page_size: 10,
            cursor: 0,
            query: "".to_string(),
            results: Vec::new(),
        }
    }

    fn reset_page(&mut self) {
        self.page = 1;
    }

    fn reset_cursor(&mut self) {
        self.cursor = 0;
    }

    fn push(&mut self, c: char) {
        self.query.push(c);
    }

    fn pop(&mut self) {
        self.query.pop();
    }

    fn next(&mut self) {
        if self.page * self.page_size <= self.results.len() {
            self.page += 1;
        }
    }

    fn back(&mut self) {
        if self.page > 1 {
            self.page -= 1;
        }
    }

    fn up(&mut self) {
        if self.cursor > 0 {
            self.cursor -= 1;
        }
    }

    fn down(&mut self) {
        if self.cursor < self.results.len() - 1 {
            self.cursor += 1;
        }
    }
}

trait ScreenRenderer {
    fn render(&mut self, state: &State);
    fn render_normal(&mut self, state: &State);
    fn render_detail(&mut self, state: &State);
}

impl ScreenRenderer for std::io::Stdout {
    fn render(&mut self, state: &State) {
        match state.mode {
            Mode::Normal => self.render_normal(state),
            Mode::Detail => self.render_detail(state),
        }
    }

    fn render_normal(&mut self, state: &State) {
        write!(self, "{}", termion::cursor::Show).unwrap();
        write!(self, "{}", termion::clear::All).unwrap();

        let (col, _row) = termion::terminal_size().unwrap();

        let mut line = String::new();
        for _ in 0..col {
            line.push('─');
        }
        write!(self, "{}", termion::cursor::Goto(1, 2)).unwrap();
        write!(self, "{}", &line).unwrap();

        for (i, result) in state.results.iter().enumerate() {
            write!(self, "{}", termion::cursor::Goto(1, 3 + i as u16)).unwrap();
            if state.cursor == i {
                write!(
                    self,
                    "{}{}{}",
                    termion::style::Underline,
                    &format!(
                        "Document {}(score: {})",
                        result.document.get("file").unwrap(),
                        result.score
                    ),
                    termion::style::Reset
                )
                .unwrap();
            } else {
                write!(
                    self,
                    "{}",
                    &format!(
                        "Document {}(score: {})",
                        result.document.get("file").unwrap(),
                        result.score
                    ),
                )
                .unwrap();
            }
        }

        let page_indicator = format!(
            "[{}/{}]",
            state.page,
            state.results.len() / state.page_size + 1
        );
        write!(
            self,
            "{}",
            termion::cursor::Goto(col - page_indicator.len() as u16, 1)
        )
        .unwrap();
        write!(self, "{}", &page_indicator).unwrap();

        write!(self, "{}", termion::cursor::Goto(1, 1)).unwrap();
        write!(self, "> ").unwrap();

        write!(self, "{}", &state.query).unwrap();

        self.flush().unwrap();
    }

    fn render_detail(&mut self, state: &State) {
        write!(self, "{}", termion::cursor::Hide).unwrap();
        write!(self, "{}", termion::clear::All).unwrap();

        let mut line = String::new();
        let (col, _row) = termion::terminal_size().unwrap();
        for _ in 0..col {
            line.push('─');
        }
        write!(self, "{}", termion::cursor::Goto(1, 2)).unwrap();
        write!(self, "{}", &line).unwrap();

        let result = state.results[state.cursor].clone();

        for (i, fragment) in result.fragments.iter().enumerate() {
            write!(self, "{}", termion::cursor::Goto(1, 3 + i as u16)).unwrap();
            write!(self, "{}", fragment,).unwrap();
        }

        write!(self, "{}", termion::cursor::Goto(1, 1)).unwrap();
        write!(
            self,
            "{}",
            &format!(
                "Document {}(score: {})",
                result.document.get("file").unwrap(),
                result.score
            )
        )
        .unwrap();

        self.flush().unwrap();
    }
}

pub async fn run(
    searcher: Box<dyn hippocampus_core::searcher::Searcher + Send + Sync>,
) -> Result<Option<hippocampus_core::searcher::SearchResult>, error::Error> {
    let stdin = std::io::stdin();
    let mut stdout = termion::screen::AlternateScreen::from(std::io::stdout()).into_raw_mode()?;
    let state = std::sync::Arc::new(std::sync::Mutex::new(State::new()));

    let (state_tx, mut state_rx) = tokio::sync::mpsc::unbounded_channel();
    let (search_result_tx, mut search_result_rx) = tokio::sync::mpsc::unbounded_channel();

    tokio::spawn(async move {
        let mut last_state = LastState::new();
        loop {
            if let Ok(Some(state)) =
                tokio::time::timeout(std::time::Duration::from_millis(10), state_rx.recv()).await
            {
                last_state.set(state);
            }
            if last_state
                .elapsed()
                .unwrap_or(std::time::Duration::from_secs(0))
                .as_millis()
                > 100
            {
                if let Some(state) = last_state.get() {
                    let results = if let Ok((_, query)) = hippocampusql::parse(&state.query) {
                        searcher
                            .search(
                                &query,
                                hippocampus_core::searcher::SearchOption {
                                    offset: (state.page - 1) * state.page_size,
                                    ..std::default::Default::default()
                                },
                            )
                            .await
                            .unwrap()
                    } else {
                        Vec::new()
                    };
                    search_result_tx.send(results).unwrap();
                    last_state.reset();
                }
            }
        }
    });

    let (event_tx, event_rx): (
        std::sync::mpsc::Sender<termion::event::Event>,
        std::sync::mpsc::Receiver<termion::event::Event>,
    ) = std::sync::mpsc::channel();

    std::thread::spawn(move || {
        for event in stdin.events().flatten() {
            event_tx.send(event).unwrap();
        }
    });

    stdout.render(&state.lock().unwrap());
    loop {
        if let Ok(event) = event_rx.try_recv() {
            let mut locked_state = state.lock().unwrap();
            match event {
                termion::event::Event::Key(termion::event::Key::Esc) => match locked_state.mode {
                    Mode::Normal => break,
                    Mode::Detail => locked_state.mode = Mode::Normal,
                },
                termion::event::Event::Key(termion::event::Key::Ctrl('c')) => {
                    break;
                }
                termion::event::Event::Key(termion::event::Key::Backspace) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.pop();
                        locked_state.reset_page();
                        locked_state.reset_cursor();
                        state_tx.send(locked_state.clone())?;
                    }
                }
                termion::event::Event::Key(termion::event::Key::Up) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.up();
                    }
                }
                termion::event::Event::Key(termion::event::Key::Down) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.down();
                    }
                }
                termion::event::Event::Key(termion::event::Key::Right) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.next();
                        locked_state.reset_cursor();
                        state_tx.send(locked_state.clone())?;
                    }
                }
                termion::event::Event::Key(termion::event::Key::Left) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.back();
                        locked_state.reset_cursor();
                        state_tx.send(locked_state.clone())?;
                    }
                }
                termion::event::Event::Key(termion::event::Key::Char('\n')) => {
                    match locked_state.mode {
                        Mode::Normal => locked_state.mode = Mode::Detail,
                        Mode::Detail => {
                            if locked_state.results.is_empty() {
                                return Ok(None);
                            }
                            return Ok(Some(locked_state.results[locked_state.cursor].clone()));
                        }
                    }
                }
                termion::event::Event::Key(termion::event::Key::Char(c)) => {
                    if let Mode::Normal = locked_state.mode {
                        locked_state.push(c);
                        locked_state.reset_page();
                        locked_state.reset_cursor();
                        state_tx.send(locked_state.clone())?;
                    }
                }
                _ => {}
            }
            stdout.render(&locked_state);
        } else if let Ok(Some(results)) = tokio::time::timeout(
            std::time::Duration::from_millis(10),
            search_result_rx.recv(),
        )
        .await
        {
            let mut locked_state = state.lock().unwrap();
            locked_state.results = results;
            stdout.render(&locked_state);
        }
    }
    Ok(None)
}
