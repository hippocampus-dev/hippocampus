#[cfg(target_os = "windows")]
mod windows;
#[cfg(target_os = "linux")]
mod x11;

#[cfg(target_os = "windows")]
pub use windows::run;
#[cfg(target_os = "linux")]
pub use x11::run;
