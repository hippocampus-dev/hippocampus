#[cfg(target_os = "linux")]
mod picker;
#[cfg(target_os = "windows")]
pub mod windows;
#[cfg(target_os = "linux")]
pub mod x11;

// The layout of the list mode of Openbox: https://github.com/Mikachu/openbox/blob/release-3.6.1/openbox/focus_cycle_popup.c
const ICON_SIZE: f32 = 36.0;
const HILITE_WIDTH: usize = 2;
const HILITE_OFFSET: usize = 3;
const OUTSIDE_BORDER: usize = 3;
const TEXT_BORDER: usize = 2;
const HILITE_RING_ALPHA: u32 = 0x88;
const HILITE_FILL_ALPHA: u32 = 0x22;
// The osd colors of the Clearlooks theme of Openbox.
const BORDER: u32 = 0xaaaaaa;
const BACKGROUND: u32 = 0xf0efee;
const ACTIVE_TEXT: u32 = 0x444444;
const INACTIVE_TEXT: u32 = 0x70747d;
