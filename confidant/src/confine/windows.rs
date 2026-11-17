// Other applications release the clip when they lose the foreground: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-clipcursor
const CLIP_INTERVAL_MILLISECONDS: u32 = 100;

/// Confines the pointer to its current monitor, re-applied on each WM_TIMER of `window`, or releases it when `confined` holds the monitor.
pub fn run(
    window: windows_sys::Win32::Foundation::HWND,
    confined: &mut Option<windows_sys::Win32::Foundation::RECT>,
) {
    if confined.take().is_some() {
        unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::KillTimer(window, 0);
            windows_sys::Win32::UI::WindowsAndMessaging::ClipCursor(std::ptr::null());
        }
        return;
    }

    let mut pointer = windows_sys::Win32::Foundation::POINT { x: 0, y: 0 };
    let mut information: windows_sys::Win32::Graphics::Gdi::MONITORINFO =
        unsafe { std::mem::zeroed() };
    information.cbSize = size_of_val(&information) as u32;
    unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::GetCursorPos(&mut pointer);
        windows_sys::Win32::Graphics::Gdi::GetMonitorInfoW(
            windows_sys::Win32::Graphics::Gdi::MonitorFromPoint(
                pointer,
                windows_sys::Win32::Graphics::Gdi::MONITOR_DEFAULTTONEAREST,
            ),
            &mut information,
        );
        windows_sys::Win32::UI::WindowsAndMessaging::ClipCursor(&information.rcMonitor);
        windows_sys::Win32::UI::WindowsAndMessaging::SetTimer(
            window,
            0,
            CLIP_INTERVAL_MILLISECONDS,
            None,
        );
    }
    *confined = Some(information.rcMonitor);
}
