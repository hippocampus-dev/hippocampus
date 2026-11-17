#![cfg_attr(windows, windows_subsystem = "windows")]

use clap::Parser;

mod confine;
mod daemon;
mod switch;

type Return = Result<(), error::Error>;

#[derive(clap::ValueEnum, Clone, Copy, Debug)]
pub enum Modifier {
    Shift,
    Control,
    Alt,
    Super,
}

/// confidant is a resident desktop utility that switches windows and confines the pointer on its own hotkeys.
#[derive(clap::Parser, Debug)]
pub struct Args {
    /// X11 keycode of the key that switches between the windows on the current monitor while held with Ctrl and Alt, backward first with Shift, Tab by default
    #[cfg(target_os = "linux")]
    #[clap(long, default_value_t = 23)]
    switch_key: u8,

    /// Virtual-key code of the key that switches between the windows on the current monitor while held with Ctrl and Alt, backward first with Shift, Tab by default
    #[cfg(target_os = "windows")]
    #[clap(long, default_value_t = 9)]
    switch_key: u16,

    /// List windows in the order they were mapped and start from the one after the focused window
    #[cfg(target_os = "linux")]
    #[clap(long)]
    linear: bool,

    /// X11 keycode of the key that confines the pointer to its current monitor and releases it on the next press, Pause by default
    #[cfg(target_os = "linux")]
    #[clap(long, default_value_t = 127)]
    confine_key: u8,

    /// Virtual-key code of the key that confines the pointer to its current monitor and releases it on the next press, Pause by default
    #[cfg(target_os = "windows")]
    #[clap(long, default_value_t = 19)]
    confine_key: u16,

    /// Modifier held with the confine key, repeated for more than one
    #[clap(long, value_enum)]
    confine_modifier: Vec<Modifier>,
}

fn main() -> Return {
    let args: Args = Args::parse();
    daemon::run(args)
}
