pub fn draw_tab<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let chunks = tui::layout::Layout::default()
        .constraints(
            [
                tui::layout::Constraint::Percentage(50),
                tui::layout::Constraint::Percentage(50),
            ]
            .as_ref(),
        )
        .direction(tui::layout::Direction::Horizontal)
        .split(area);

    draw_cpu_utilization_chart(f, state, chunks[0]);
}

pub fn draw_per_process_tab<B>(
    f: &mut tui::Frame<B>,
    state: &mut crate::ui::State,
    area: tui::layout::Rect,
) where
    B: tui::backend::Backend,
{
    let chunks = tui::layout::Layout::default()
        .constraints(
            [
                tui::layout::Constraint::Percentage(50),
                tui::layout::Constraint::Percentage(50),
            ]
            .as_ref(),
        )
        .direction(tui::layout::Direction::Vertical)
        .split(area);
    draw_cpu_usage_bar_chart(f, state, chunks[0]);

    let chunks = tui::layout::Layout::default()
        .constraints(
            [
                tui::layout::Constraint::Percentage(50),
                tui::layout::Constraint::Percentage(50),
            ]
            .as_ref(),
        )
        .direction(tui::layout::Direction::Horizontal)
        .split(chunks[1]);
    draw_cpu_utilization_chart(f, state, chunks[0]);
}

fn draw_cpu_usage_bar_chart<B>(
    f: &mut tui::Frame<B>,
    state: &mut crate::ui::State,
    area: tui::layout::Rect,
) where
    B: tui::backend::Backend,
{
    let mut v = Vec::new();
    for (_k, histogram) in &state.cpu_usage {
        let t: Vec<(String, u32)> = histogram.into();
        v.extend(t);
    }
    f.render_widget(
        tui::widgets::BarChart::default()
            .block(
                tui::widgets::Block::default()
                    .borders(tui::widgets::Borders::ALL)
                    .title("CPU Usage"),
            )
            .data(
                &v.iter()
                    .map(|(s, val)| (s as &str, *val as u64))
                    .collect::<Vec<(&str, u64)>>(),
            )
            .bar_width(10)
            .bar_gap(2)
            .bar_set(tui::symbols::bar::NINE_LEVELS)
            .value_style(
                tui::style::Style::default()
                    .fg(tui::style::Color::Black)
                    .bg(tui::style::Color::Green)
                    .add_modifier(tui::style::Modifier::ITALIC),
            )
            .label_style(tui::style::Style::default().fg(tui::style::Color::Yellow))
            .bar_style(tui::style::Style::default().fg(tui::style::Color::Green)),
        area,
    );
}

fn draw_cpu_utilization_chart<B>(
    f: &mut tui::Frame<B>,
    state: &mut crate::ui::State,
    area: tui::layout::Rect,
) where
    B: tui::backend::Backend,
{
    let cpu_utilization = state
        .cpu_utilization
        .iter()
        .enumerate()
        .map(|(i, v)| (((i + 1) as f64), *v))
        .collect::<Vec<(f64, f64)>>();
    let chart = tui::widgets::Chart::new(vec![tui::widgets::Dataset::default()
        .marker(tui::symbols::Marker::Dot)
        .style(tui::style::Style::default().fg(tui::style::Color::Cyan))
        .data(&cpu_utilization)])
    .block(
        tui::widgets::Block::default()
            .borders(tui::widgets::Borders::ALL)
            .title("CPU %"),
    )
    .x_axis(
        tui::widgets::Axis::default()
            .style(tui::style::Style::default().fg(tui::style::Color::Gray))
            .bounds([1.0, crate::ui::CHART_HISTORY_LIMIT as f64])
            .labels(vec![
                tui::text::Span::raw(""),
                tui::text::Span::styled(
                    format!(
                        "Utilization: {:.2}%",
                        if !cpu_utilization.is_empty() {
                            cpu_utilization[cpu_utilization.len() - 1].1
                        } else {
                            0.0
                        }
                    ),
                    tui::style::Style::default().fg(tui::style::Color::Cyan),
                ),
                tui::text::Span::raw(""),
            ]),
    )
    .y_axis(
        tui::widgets::Axis::default()
            .style(tui::style::Style::default().fg(tui::style::Color::Gray))
            .bounds(
                if !cpu_utilization.is_empty()
                    && cpu_utilization[cpu_utilization.len() - 1].1 > 100.0
                {
                    [0.0, 100.0 * num_cpus::get() as f64]
                } else {
                    [0.0, 100.0]
                },
            )
            .labels(vec![
                tui::text::Span::styled(
                    "0",
                    tui::style::Style::default().add_modifier(tui::style::Modifier::BOLD),
                ),
                tui::text::Span::raw(
                    if !cpu_utilization.is_empty()
                        && cpu_utilization[cpu_utilization.len() - 1].1 > 100.0
                    {
                        (100 * num_cpus::get() / 2).to_string()
                    } else {
                        "50".to_string()
                    },
                ),
                tui::text::Span::styled(
                    if !cpu_utilization.is_empty()
                        && cpu_utilization[cpu_utilization.len() - 1].1 > 100.0
                    {
                        (100 * num_cpus::get()).to_string()
                    } else {
                        "100".to_string()
                    },
                    tui::style::Style::default().add_modifier(tui::style::Modifier::BOLD),
                ),
            ]),
    );
    f.render_widget(chart, area);
}
