pub fn draw_tab<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let trace_pipe_lines_len = state.trace_pipe.text.lines.len() as u16;
    f.render_widget(
        tui::widgets::Paragraph::new(state.trace_pipe.text.clone())
            .block(
                tui::widgets::Block::default()
                    .borders(tui::widgets::Borders::ALL)
                    .title("TracePipe"),
            )
            // https://github.com/fdehau/tui-rs/pull/349
            // .wrap(tui::widgets::Wrap { trim: false })
            .scroll((
                if trace_pipe_lines_len > area.height {
                    if state.trace_pipe.follow
                        || state.trace_pipe.scroll.0
                            > trace_pipe_lines_len - area.height + crate::ui::LINE_OFFSET
                    {
                        state.trace_pipe.follow = true;
                        let actual = trace_pipe_lines_len - area.height + crate::ui::LINE_OFFSET;
                        state.trace_pipe.scroll = (actual, state.trace_pipe.scroll.1);
                        actual
                    } else {
                        state.trace_pipe.scroll.0
                    }
                } else {
                    let actual = 0;
                    state.trace_pipe.scroll = (actual, state.trace_pipe.scroll.1);
                    actual
                },
                state.trace_pipe.scroll.1,
            )),
        area,
    );
}
