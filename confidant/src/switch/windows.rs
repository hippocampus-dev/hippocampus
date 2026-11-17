pub const MESSAGE_OPEN: u32 = windows_sys::Win32::UI::WindowsAndMessaging::WM_APP;
pub const MESSAGE_ADVANCE: u32 = MESSAGE_OPEN + 1;
pub const MESSAGE_CANCEL: u32 = MESSAGE_OPEN + 2;
pub const MESSAGE_COMMIT: u32 = MESSAGE_OPEN + 3;
// Pressing an unassigned key keeps the release of Alt from opening the menu bar: https://www.autohotkey.com/docs/v2/lib/A_MenuMaskKey.htm
const MENU_MASK_KEY: u16 = 0xE8;
const ICON_TIMEOUT_MILLISECONDS: u32 = 100;

/// Set by the keyboard hook when it opens the list and cleared once the list closes.
pub static OPEN: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

struct Entry {
    id: windows_sys::Win32::Foundation::HWND,
    title: Vec<u16>,
    icon: windows_sys::Win32::UI::WindowsAndMessaging::HICON,
    topmost: bool,
}

struct State {
    entries: Vec<Entry>,
    selected: usize,
    font: windows_sys::Win32::Graphics::Gdi::HFONT,
    icon_size: i32,
    row_height: i32,
}

thread_local! {
    static STATE: std::cell::RefCell<State> = const {
        std::cell::RefCell::new(State {
            entries: Vec::new(),
            selected: 0,
            font: std::ptr::null_mut(),
            icon_size: 0,
            row_height: 0,
        })
    };
}

/// Converts 0xRRGGBB, blended over the background by `alpha` out of 0xff, into a COLORREF.
fn color(rgb: u32, alpha: u32) -> u32 {
    let channel = |shift: u32| {
        let foreground = rgb >> shift & 0xff;
        let background = super::BACKGROUND >> shift & 0xff;
        (background * (0xff - alpha) + foreground * alpha) / 0xff
    };
    channel(16) | channel(8) << 8 | channel(0) << 16
}

fn send_menu_mask() {
    let input = |flags| windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT {
        r#type: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_KEYBOARD,
        Anonymous: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_0 {
            ki: windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYBDINPUT {
                wVk: MENU_MASK_KEY,
                wScan: 0,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    };
    let inputs = [
        input(0),
        input(windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYEVENTF_KEYUP),
    ];
    unsafe {
        windows_sys::Win32::UI::Input::KeyboardAndMouse::SendInput(
            inputs.len() as u32,
            inputs.as_ptr(),
            size_of::<windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT>() as i32,
        );
    }
}

unsafe extern "system" fn collect(
    window: windows_sys::Win32::Foundation::HWND,
    data: windows_sys::Win32::Foundation::LPARAM,
) -> windows_sys::core::BOOL {
    let entries = unsafe { &mut *(data as *mut Vec<Entry>) };
    let mut cloaked = 0u32;
    let style = unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::GetWindowLongPtrW(
            window,
            windows_sys::Win32::UI::WindowsAndMessaging::GWL_EXSTYLE,
        )
    } as u32;
    let selectable = unsafe {
        windows_sys::Win32::Graphics::Dwm::DwmGetWindowAttribute(
            window,
            windows_sys::Win32::Graphics::Dwm::DWMWA_CLOAKED as u32,
            &mut cloaked as *mut u32 as *mut std::ffi::c_void,
            size_of::<u32>() as u32,
        );
        windows_sys::Win32::UI::WindowsAndMessaging::IsWindowVisible(window) != 0
            && cloaked == 0
            && (windows_sys::Win32::UI::WindowsAndMessaging::GetWindow(
                window,
                windows_sys::Win32::UI::WindowsAndMessaging::GW_OWNER,
            )
            .is_null()
                || style & windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_APPWINDOW != 0)
            && style & windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_TOOLWINDOW == 0
    };
    if selectable {
        let length =
            unsafe { windows_sys::Win32::UI::WindowsAndMessaging::GetWindowTextLengthW(window) };
        let mut title = vec![0u16; length as usize + 1];
        let length = unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::GetWindowTextW(
                window,
                title.as_mut_ptr(),
                title.len() as i32,
            )
        };
        title.truncate(length as usize);
        if !title.is_empty() {
            let mut icon = 0;
            unsafe {
                windows_sys::Win32::UI::WindowsAndMessaging::SendMessageTimeoutW(
                    window,
                    windows_sys::Win32::UI::WindowsAndMessaging::WM_GETICON,
                    windows_sys::Win32::UI::WindowsAndMessaging::ICON_BIG as usize,
                    0,
                    windows_sys::Win32::UI::WindowsAndMessaging::SMTO_ABORTIFHUNG,
                    ICON_TIMEOUT_MILLISECONDS,
                    &mut icon,
                );
            }
            if icon == 0 {
                icon = unsafe {
                    windows_sys::Win32::UI::WindowsAndMessaging::GetClassLongPtrW(
                        window,
                        windows_sys::Win32::UI::WindowsAndMessaging::GCLP_HICON,
                    )
                };
            }
            entries.push(Entry {
                id: window,
                title,
                icon: icon as windows_sys::Win32::UI::WindowsAndMessaging::HICON,
                topmost: style & windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_TOPMOST != 0,
            });
        }
    }
    1
}

fn paint(window: windows_sys::Win32::Foundation::HWND) {
    STATE.with_borrow(|state| {
        let mut paint: windows_sys::Win32::Graphics::Gdi::PAINTSTRUCT =
            unsafe { std::mem::zeroed() };
        let mut client = windows_sys::Win32::Foundation::RECT {
            left: 0,
            top: 0,
            right: 0,
            bottom: 0,
        };
        unsafe {
            let context = windows_sys::Win32::Graphics::Gdi::BeginPaint(window, &mut paint);
            windows_sys::Win32::UI::WindowsAndMessaging::GetClientRect(window, &mut client);
            windows_sys::Win32::Graphics::Gdi::SelectObject(context, state.font);
            windows_sys::Win32::Graphics::Gdi::SetBkMode(
                context,
                windows_sys::Win32::Graphics::Gdi::TRANSPARENT as i32,
            );
            let fill = |rectangle: &windows_sys::Win32::Foundation::RECT, color| {
                let brush = windows_sys::Win32::Graphics::Gdi::CreateSolidBrush(color);
                windows_sys::Win32::Graphics::Gdi::FillRect(context, rectangle, brush);
                windows_sys::Win32::Graphics::Gdi::DeleteObject(brush);
            };
            fill(&client, color(super::BORDER, 0xff));
            let inside = windows_sys::Win32::Foundation::RECT {
                left: 1,
                top: 1,
                right: client.right - 1,
                bottom: client.bottom - 1,
            };
            fill(&inside, color(super::BACKGROUND, 0xff));
            let outside = 1 + super::OUTSIDE_BORDER as i32;
            let offset = super::HILITE_OFFSET as i32;
            let hilite_size = state.icon_size + offset * 2;
            for (row, entry) in state.entries.iter().enumerate() {
                let top = outside + state.row_height * row as i32;
                let hilite_top = top + (state.row_height - hilite_size) / 2;
                if row == state.selected {
                    let ring = super::HILITE_WIDTH as i32;
                    fill(
                        &windows_sys::Win32::Foundation::RECT {
                            left: outside,
                            top: hilite_top,
                            right: outside + hilite_size,
                            bottom: hilite_top + hilite_size,
                        },
                        color(super::ACTIVE_TEXT, super::HILITE_RING_ALPHA),
                    );
                    fill(
                        &windows_sys::Win32::Foundation::RECT {
                            left: outside + ring,
                            top: hilite_top + ring,
                            right: outside + hilite_size - ring,
                            bottom: hilite_top + hilite_size - ring,
                        },
                        color(super::ACTIVE_TEXT, super::HILITE_FILL_ALPHA),
                    );
                }
                if !entry.icon.is_null() {
                    windows_sys::Win32::UI::WindowsAndMessaging::DrawIconEx(
                        context,
                        outside + offset,
                        hilite_top + offset,
                        entry.icon,
                        state.icon_size,
                        state.icon_size,
                        0,
                        std::ptr::null_mut(),
                        windows_sys::Win32::UI::WindowsAndMessaging::DI_NORMAL,
                    );
                }
                windows_sys::Win32::Graphics::Gdi::SetTextColor(
                    context,
                    if row == state.selected {
                        color(super::ACTIVE_TEXT, 0xff)
                    } else {
                        color(super::INACTIVE_TEXT, 0xff)
                    },
                );
                let mut text = windows_sys::Win32::Foundation::RECT {
                    left: outside * 2 - 1 + hilite_size + super::TEXT_BORDER as i32,
                    top,
                    right: client.right - outside - super::TEXT_BORDER as i32,
                    bottom: top + state.row_height,
                };
                windows_sys::Win32::Graphics::Gdi::DrawTextW(
                    context,
                    entry.title.as_ptr(),
                    entry.title.len() as i32,
                    &mut text,
                    windows_sys::Win32::Graphics::Gdi::DT_SINGLELINE
                        | windows_sys::Win32::Graphics::Gdi::DT_VCENTER
                        | windows_sys::Win32::Graphics::Gdi::DT_END_ELLIPSIS
                        | windows_sys::Win32::Graphics::Gdi::DT_NOPREFIX,
                );
            }
            windows_sys::Win32::Graphics::Gdi::EndPaint(window, &paint);
        }
    });
}

unsafe extern "system" fn procedure(
    window: windows_sys::Win32::Foundation::HWND,
    message: u32,
    w_param: windows_sys::Win32::Foundation::WPARAM,
    l_param: windows_sys::Win32::Foundation::LPARAM,
) -> windows_sys::Win32::Foundation::LRESULT {
    match message {
        windows_sys::Win32::UI::WindowsAndMessaging::WM_PAINT => paint(window),
        _ => {
            return unsafe {
                windows_sys::Win32::UI::WindowsAndMessaging::DefWindowProcW(
                    window, message, w_param, l_param,
                )
            };
        }
    }
    0
}

fn activate(id: windows_sys::Win32::Foundation::HWND) {
    send_menu_mask();
    let switched = unsafe {
        if windows_sys::Win32::UI::WindowsAndMessaging::IsIconic(id) != 0 {
            windows_sys::Win32::UI::WindowsAndMessaging::ShowWindowAsync(
                id,
                windows_sys::Win32::UI::WindowsAndMessaging::SW_RESTORE,
            );
        }
        windows_sys::Win32::UI::WindowsAndMessaging::SetForegroundWindow(id)
    };
    if switched == 0 {
        eprintln!("failed to bring {id:?} to the foreground");
    }
}

/// Creates the hidden window of the list, or returns None when another instance already has one.
pub fn create() -> Result<Option<windows_sys::Win32::Foundation::HWND>, error::Error> {
    let class_name = "confidant"
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    let running = unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::FindWindowW(
            class_name.as_ptr(),
            std::ptr::null(),
        )
    };
    if !running.is_null() {
        return Ok(None);
    }

    let module =
        unsafe { windows_sys::Win32::System::LibraryLoader::GetModuleHandleW(std::ptr::null()) };
    let mut class: windows_sys::Win32::UI::WindowsAndMessaging::WNDCLASSW =
        unsafe { std::mem::zeroed() };
    class.lpfnWndProc = Some(procedure);
    class.hInstance = module;
    class.lpszClassName = class_name.as_ptr();
    let window = unsafe {
        windows_sys::Win32::UI::HiDpi::SetProcessDpiAwarenessContext(
            windows_sys::Win32::UI::HiDpi::DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2,
        );
        windows_sys::Win32::UI::WindowsAndMessaging::RegisterClassW(&class);
        windows_sys::Win32::UI::WindowsAndMessaging::CreateWindowExW(
            windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_NOACTIVATE
                | windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_TOOLWINDOW
                | windows_sys::Win32::UI::WindowsAndMessaging::WS_EX_TOPMOST,
            class_name.as_ptr(),
            class_name.as_ptr(),
            windows_sys::Win32::UI::WindowsAndMessaging::WS_POPUP,
            0,
            0,
            0,
            0,
            std::ptr::null_mut(),
            std::ptr::null_mut(),
            module,
            std::ptr::null(),
        )
    };
    if window.is_null() {
        return Err(std::io::Error::last_os_error().into());
    }
    Ok(Some(window))
}

/// Opens the list on the monitor of the foreground window, or switches at once when Alt is already released.
pub fn open(window: windows_sys::Win32::Foundation::HWND, reverse: bool) {
    let mut entries = Vec::new();
    unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::EnumWindows(
            Some(collect),
            &mut entries as *mut Vec<Entry> as windows_sys::Win32::Foundation::LPARAM,
        );
    }
    let foreground = unsafe { windows_sys::Win32::UI::WindowsAndMessaging::GetForegroundWindow() };
    let monitor_of = |id| unsafe {
        windows_sys::Win32::Graphics::Gdi::MonitorFromWindow(
            id,
            windows_sys::Win32::Graphics::Gdi::MONITOR_DEFAULTTONEAREST,
        )
    };
    let monitor = if entries.iter().any(|entry: &Entry| entry.id == foreground) {
        monitor_of(foreground)
    } else {
        let mut pointer = windows_sys::Win32::Foundation::POINT { x: 0, y: 0 };
        unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::GetCursorPos(&mut pointer);
            windows_sys::Win32::Graphics::Gdi::MonitorFromPoint(
                pointer,
                windows_sys::Win32::Graphics::Gdi::MONITOR_DEFAULTTONEAREST,
            )
        }
    };
    entries.retain(|entry| monitor_of(entry.id) == monitor);
    // The Z order keeps these on top however long ago they were used.
    entries.sort_by_key(|entry| (entry.id != foreground, entry.topmost));
    if entries.iter().all(|entry| entry.id == foreground) {
        OPEN.store(false, std::sync::atomic::Ordering::SeqCst);
        return;
    }

    let length = entries.len();
    let current = entries.iter().position(|entry| entry.id == foreground);
    let selected = if reverse {
        current.map_or(length - 1, |index| (index + length - 1) % length)
    } else {
        current.map_or(0, |index| (index + 1) % length)
    };
    let alt = unsafe {
        windows_sys::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState(
            windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_MENU as i32,
        )
    } < 0;
    if !alt {
        activate(entries[selected].id);
        OPEN.store(false, std::sync::atomic::Ordering::SeqCst);
        return;
    }
    send_menu_mask();
    STATE.with_borrow_mut(|state| {
        state.entries = entries;
        state.selected = selected;
        let mut dots_per_inch = 0;
        let mut unused = 0;
        let mut metrics: windows_sys::Win32::UI::WindowsAndMessaging::NONCLIENTMETRICSW =
            unsafe { std::mem::zeroed() };
        metrics.cbSize = size_of_val(&metrics) as u32;
        let mut information: windows_sys::Win32::Graphics::Gdi::MONITORINFO =
            unsafe { std::mem::zeroed() };
        information.cbSize = size_of_val(&information) as u32;
        unsafe {
            windows_sys::Win32::UI::HiDpi::GetDpiForMonitor(
                monitor,
                windows_sys::Win32::UI::HiDpi::MDT_EFFECTIVE_DPI,
                &mut dots_per_inch,
                &mut unused,
            );
            windows_sys::Win32::UI::HiDpi::SystemParametersInfoForDpi(
                windows_sys::Win32::UI::WindowsAndMessaging::SPI_GETNONCLIENTMETRICS,
                metrics.cbSize,
                &mut metrics as *mut _ as *mut std::ffi::c_void,
                0,
                dots_per_inch,
            );
            windows_sys::Win32::Graphics::Gdi::GetMonitorInfoW(monitor, &mut information);
            metrics.lfMessageFont.lfWeight = windows_sys::Win32::Graphics::Gdi::FW_BOLD as i32;
            windows_sys::Win32::Graphics::Gdi::DeleteObject(state.font);
            state.font =
                windows_sys::Win32::Graphics::Gdi::CreateFontIndirectW(&metrics.lfMessageFont);
        }
        state.icon_size = (super::ICON_SIZE * dots_per_inch as f32
            / windows_sys::Win32::UI::WindowsAndMessaging::USER_DEFAULT_SCREEN_DPI as f32)
            as i32;
        let hilite_size = state.icon_size + super::HILITE_OFFSET as i32 * 2;

        let bounds = information.rcWork;
        let context = unsafe { windows_sys::Win32::Graphics::Gdi::GetDC(window) };
        unsafe {
            windows_sys::Win32::Graphics::Gdi::SelectObject(context, state.font);
        }
        let mut width = 0;
        for entry in &state.entries {
            let mut size = windows_sys::Win32::Foundation::SIZE { cx: 0, cy: 0 };
            unsafe {
                windows_sys::Win32::Graphics::Gdi::GetTextExtentPoint32W(
                    context,
                    entry.title.as_ptr(),
                    entry.title.len() as i32,
                    &mut size,
                );
            }
            width = width.max(size.cx);
            state.row_height = (size.cy + super::TEXT_BORDER as i32 * 2).max(hilite_size);
        }
        unsafe {
            windows_sys::Win32::Graphics::Gdi::ReleaseDC(window, context);
        }
        // Each side also carries the 1 pixel border.
        let width = (2
            + super::OUTSIDE_BORDER as i32 * 3
            + hilite_size
            + super::TEXT_BORDER as i32 * 2
            + width)
            .min((bounds.right - bounds.left) / 3);
        let height =
            2 + super::OUTSIDE_BORDER as i32 * 2 + state.row_height * state.entries.len() as i32;
        unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::SetWindowPos(
                window,
                windows_sys::Win32::UI::WindowsAndMessaging::HWND_TOPMOST,
                (bounds.left + bounds.right - width) / 2,
                (bounds.top + bounds.bottom - height) / 2,
                width,
                height,
                windows_sys::Win32::UI::WindowsAndMessaging::SWP_NOACTIVATE
                    | windows_sys::Win32::UI::WindowsAndMessaging::SWP_SHOWWINDOW,
            );
        }
    });
}

/// Moves the selection of the open list, which the keyboard hook can ask for before the list opens.
pub fn advance(window: windows_sys::Win32::Foundation::HWND, backward: bool) {
    if unsafe { windows_sys::Win32::UI::WindowsAndMessaging::IsWindowVisible(window) } == 0 {
        return;
    }
    if unsafe {
        windows_sys::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState(
            windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_MENU as i32,
        )
    } >= 0
    {
        close(window, true);
        return;
    }
    STATE.with_borrow_mut(|state| {
        let length = state.entries.len();
        state.selected = if backward {
            (state.selected + length - 1) % length
        } else {
            (state.selected + 1) % length
        };
        unsafe {
            windows_sys::Win32::Graphics::Gdi::InvalidateRect(window, std::ptr::null(), 0);
        }
    });
}

/// Closes the open list, switching to the selected window when `commit` is set.
pub fn close(window: windows_sys::Win32::Foundation::HWND, commit: bool) {
    if unsafe { windows_sys::Win32::UI::WindowsAndMessaging::IsWindowVisible(window) } == 0 {
        return;
    }
    unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::ShowWindow(
            window,
            windows_sys::Win32::UI::WindowsAndMessaging::SW_HIDE,
        );
    }
    OPEN.store(false, std::sync::atomic::Ordering::SeqCst);
    if commit {
        STATE.with_borrow(|state| activate(state.entries[state.selected].id));
    }
}
