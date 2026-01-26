pub const LINE_OFFSET: u16 = 2;
const CHUNK_HISTORY_LIMIT: usize = 10000;
pub const CHART_HISTORY_LIMIT: usize = 50;

mod network;
mod resource;
mod trace;
pub mod vfs;

#[derive(Clone, Debug)]
pub struct Chunk<'a> {
    text: tui::text::Text<'a>,
    scroll: (u16, u16),
    follow: bool,
}

impl<'a> Default for Chunk<'a> {
    fn default() -> Self {
        Self {
            text: tui::text::Text::default(),
            scroll: (0, 0),
            follow: true,
        }
    }
}

impl<'a> Extend<tui::text::Spans<'a>> for Chunk<'a> {
    fn extend<T>(&mut self, iter: T)
    where
        T: IntoIterator<Item = tui::text::Spans<'a>>,
    {
        self.text.extend(iter);
        if self.text.height() > CHUNK_HISTORY_LIMIT {
            let lines =
                &self.text.lines[(self.text.height() - CHUNK_HISTORY_LIMIT)..self.text.height()];
            self.text.lines = lines.to_vec();
        }
    }
}

impl<'a> Chunk<'a> {
    fn on_up(&mut self) {
        self.follow = false;
        self.scroll = (self.scroll.0.saturating_sub(1), self.scroll.1);
    }

    fn on_down(&mut self) {
        self.scroll = (self.scroll.0.saturating_add(1), self.scroll.1);
    }
}

#[derive(
    Clone,
    Debug,
    Default,
    num_derive::FromPrimitive,
    num_derive::ToPrimitive,
    enum_derive::EnumLen,
    enum_derive::EnumIter,
    enum_derive::EnumToString,
)]
pub enum TabType {
    #[default]
    Network,
    Resource,
    Vfs,
    #[cfg(debug_assertions)]
    TracePipe,
}

#[derive(Clone, Debug, Default)]
pub struct Tab {
    len: usize,
    index: TabType,
}

impl Tab {
    fn with_len(len: usize) -> Self {
        Self {
            len,
            index: TabType::Network,
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct State<'a> {
    args: crate::Args,
    titles: Vec<String>,
    pub l7: Chunk<'a>,
    pub cpu_usage: std::collections::HashMap<String, crate::core::types::Histogram>,
    pub cpu_utilization: std::collections::VecDeque<f64>,
    pub trace_pipe: Chunk<'a>,
    pub vfs_events: Chunk<'a>,
    pub vfs_stats_table: vfs::VfsStatsTable,
    pub tab: Tab,
}

impl<'a> State<'a> {
    pub fn new(args: crate::Args) -> Self {
        Self {
            args,
            titles: TabType::iter().map(|e| e.to_string()).collect(),
            l7: Default::default(),
            cpu_usage: Default::default(),
            cpu_utilization: std::collections::VecDeque::with_capacity(CHART_HISTORY_LIMIT),
            trace_pipe: Default::default(),
            vfs_events: Default::default(),
            vfs_stats_table: Default::default(),
            tab: Tab::with_len(TabType::len()),
        }
    }

    pub fn next_tab(&mut self) {
        if let Some(tab_type) = num_traits::ToPrimitive::to_usize(&self.tab.index)
            .and_then(|size| num_traits::FromPrimitive::from_usize((size + 1) % self.tab.len))
        {
            self.tab.index = tab_type;
        }
    }

    pub fn previous_tab(&mut self) {
        if let Some(tab_type) = num_traits::ToPrimitive::to_usize(&self.tab.index)
            .and_then(|size| num_traits::FromPrimitive::from_usize((size - 1) % self.tab.len))
        {
            self.tab.index = tab_type;
        }
    }

    pub fn on_up(&mut self) {
        match self.tab.index {
            TabType::Network => {
                self.l7.on_up();
            }
            TabType::Resource => {}
            TabType::Vfs => {
                self.vfs_stats_table.on_up();
                self.vfs_events.on_up();
            }
            #[allow(unreachable_patterns)]
            _ => {
                self.trace_pipe.on_up();
            }
        }
    }

    pub fn on_down(&mut self) {
        match self.tab.index {
            TabType::Network => {
                self.l7.on_down();
            }
            TabType::Resource => {}
            TabType::Vfs => {
                self.vfs_stats_table.on_down();
                self.vfs_events.on_down();
            }
            #[allow(unreachable_patterns)]
            _ => {
                self.trace_pipe.on_down();
            }
        }
    }

    pub fn bottom(&mut self) {
        match self.tab.index {
            TabType::Network => {
                self.l7.scroll = (u16::MAX, self.l7.scroll.1);
            }
            TabType::Resource => {}
            TabType::Vfs => {
                self.vfs_events.scroll = (u16::MAX, self.vfs_events.scroll.1);
            }
            #[allow(unreachable_patterns)]
            _ => {
                self.trace_pipe.scroll = (u16::MAX, self.trace_pipe.scroll.1);
            }
        }
    }
}

pub fn draw<B>(f: &mut tui::Frame<B>, state: &mut State)
where
    B: tui::backend::Backend,
{
    let chunks = tui::layout::Layout::default()
        .constraints(
            [
                tui::layout::Constraint::Length(1),
                tui::layout::Constraint::Min(0),
            ]
            .as_ref(),
        )
        .split(f.size());
    let titles = state
        .titles
        .iter()
        .map(|t| {
            tui::text::Spans::from(tui::text::Span::styled(
                t,
                tui::style::Style::default().fg(tui::style::Color::Green),
            ))
        })
        .collect();
    let tabs = tui::widgets::Tabs::new(titles)
        .block(tui::widgets::Block::default())
        .highlight_style(tui::style::Style::default().fg(tui::style::Color::Yellow))
        .select(num_traits::ToPrimitive::to_usize(&state.tab.index).unwrap_or_default());
    f.render_widget(tabs, chunks[0]);
    match state.tab.index {
        TabType::Network => network::draw_tab(f, state, chunks[1]),
        TabType::Resource => {
            if state.args.tgid.is_some() {
                resource::draw_per_process_tab(f, state, chunks[1])
            } else {
                resource::draw_tab(f, state, chunks[1])
            }
        }
        TabType::Vfs => vfs::draw_tab(f, state, chunks[1]),
        #[allow(unreachable_patterns)]
        _ => trace::draw_tab(f, state, chunks[1]),
    }
}
