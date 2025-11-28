pub fn draw_tab<B>(f: &mut tui::Frame<B>, state: &mut crate::ui::State, area: tui::layout::Rect)
where
    B: tui::backend::Backend,
{
    let l7_lines_len = state.l7.text.lines.len() as u16;
    f.render_widget(
        tui::widgets::Paragraph::new(state.l7.text.clone())
            .block(
                tui::widgets::Block::default()
                    .borders(tui::widgets::Borders::ALL)
                    .title("L7"),
            )
            // https://github.com/fdehau/tui-rs/pull/349
            // .wrap(tui::widgets::Wrap { trim: false })
            .scroll((
                if l7_lines_len > area.height {
                    if state.l7.follow
                        || state.l7.scroll.0 >= l7_lines_len - area.height + crate::ui::LINE_OFFSET
                    {
                        state.l7.follow = true;
                        let actual = l7_lines_len - area.height + crate::ui::LINE_OFFSET;
                        state.l7.scroll = (actual, state.l7.scroll.1);
                        actual
                    } else {
                        state.l7.scroll.0
                    }
                } else {
                    let actual = 0;
                    state.l7.scroll = (actual, state.l7.scroll.1);
                    actual
                },
                state.l7.scroll.1,
            )),
        area,
    );
}
