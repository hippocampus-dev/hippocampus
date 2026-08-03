#[derive(Clone, Debug, Default)]
pub struct VfsFileStats {
    pub filename: String,
    pub read_count: usize,
    pub write_count: usize,
    pub read_bytes: u64,
    pub write_bytes: u64,
}

impl VfsFileStats {
    pub fn new(filename: String) -> Self {
        Self {
            filename,
            read_count: 0,
            write_count: 0,
            read_bytes: 0,
            write_bytes: 0,
        }
    }

    pub fn update(&mut self, op: &crate::vfs::FileOp, size: u64) {
        match op {
            crate::vfs::FileOp::Read => {
                self.read_count += 1;
                self.read_bytes += size;
            }
            crate::vfs::FileOp::Write => {
                self.write_count += 1;
                self.write_bytes += size;
            }
            crate::vfs::FileOp::Unknown(_) => {}
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct VfsStatsTable {
    pub stats: std::collections::HashMap<String, VfsFileStats>,
    table_state: tui::widgets::TableState,
}

impl VfsStatsTable {
    pub fn on_up(&mut self) {
        let i = match self.table_state.selected() {
            Some(i) => {
                if i == 0 {
                    self.stats.len().saturating_sub(1)
                } else {
                    i - 1
                }
            }
            None => 0,
        };
        self.table_state.select(Some(i));
    }

    pub fn on_down(&mut self) {
        let i = match self.table_state.selected() {
            Some(i) => {
                if i >= self.stats.len().saturating_sub(1) {
                    0
                } else {
                    i + 1
                }
            }
            None => 0,
        };
        self.table_state.select(Some(i));
    }
}

pub fn draw_tab<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let chunks = tui::layout::Layout::default()
        .direction(tui::layout::Direction::Vertical)
        .constraints(
            [
                tui::layout::Constraint::Percentage(50),
                tui::layout::Constraint::Percentage(50),
            ]
            .as_ref(),
        )
        .split(area);

    draw_stats_table(f, state, chunks[0]);
    draw_events_log(f, state, chunks[1]);
}

fn draw_stats_table<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let mut stats_vec: Vec<&VfsFileStats> = state.vfs_stats_table.stats.values().collect();
    stats_vec.sort_by(|a, b| (b.read_bytes + b.write_bytes).cmp(&(a.read_bytes + a.write_bytes)));

    let header_cells = ["FILE", "READS", "READ BYTES", "WRITES", "WRITE BYTES"]
        .iter()
        .map(|h| {
            tui::widgets::Cell::from(*h).style(
                tui::style::Style::default()
                    .fg(tui::style::Color::Yellow)
                    .add_modifier(tui::style::Modifier::BOLD),
            )
        });
    let header = tui::widgets::Row::new(header_cells).height(1);

    let rows = stats_vec.iter().map(|stats| {
        let cells = vec![
            tui::widgets::Cell::from(stats.filename.clone()),
            tui::widgets::Cell::from(format!("{}", stats.read_count)),
            tui::widgets::Cell::from(format_bytes(stats.read_bytes)),
            tui::widgets::Cell::from(format!("{}", stats.write_count)),
            tui::widgets::Cell::from(format_bytes(stats.write_bytes)),
        ];
        tui::widgets::Row::new(cells).height(1)
    });

    let table = tui::widgets::Table::new(rows)
        .header(header)
        .block(
            tui::widgets::Block::default()
                .borders(tui::widgets::Borders::ALL)
                .title("VFS Statistics (by file)"),
        )
        .widths(&[
            tui::layout::Constraint::Percentage(40),
            tui::layout::Constraint::Percentage(15),
            tui::layout::Constraint::Percentage(15),
            tui::layout::Constraint::Percentage(15),
            tui::layout::Constraint::Percentage(15),
        ])
        .highlight_style(tui::style::Style::default().add_modifier(tui::style::Modifier::REVERSED));

    f.render_stateful_widget(table, area, &mut state.vfs_stats_table.table_state);
}

fn draw_events_log<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let events_len = state.vfs_events.text.lines.len() as u16;
    f.render_widget(
        tui::widgets::Paragraph::new(state.vfs_events.text.clone())
            .block(
                tui::widgets::Block::default()
                    .borders(tui::widgets::Borders::ALL)
                    .title("VFS Events"),
            )
            .scroll((
                if events_len > area.height {
                    if state.vfs_events.follow
                        || state.vfs_events.scroll.0
                            >= events_len - area.height + crate::ui::LINE_OFFSET
                    {
                        state.vfs_events.follow = true;
                        let actual = events_len - area.height + crate::ui::LINE_OFFSET;
                        state.vfs_events.scroll = (actual, state.vfs_events.scroll.1);
                        actual
                    } else {
                        state.vfs_events.scroll.0
                    }
                } else {
                    let actual = 0;
                    state.vfs_events.scroll = (actual, state.vfs_events.scroll.1);
                    actual
                },
                state.vfs_events.scroll.1,
            )),
        area,
    );
}

fn format_bytes(bytes: u64) -> String {
    const KB: u64 = 1024;
    const MB: u64 = KB * 1024;
    const GB: u64 = MB * 1024;

    if bytes >= GB {
        format!("{:.2} GB", bytes as f64 / GB as f64)
    } else if bytes >= MB {
        format!("{:.2} MB", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{:.2} KB", bytes as f64 / KB as f64)
    } else {
        format!("{bytes} B")
    }
}
