const MESSAGE_CONFINE: u32 = crate::switch::windows::MESSAGE_COMMIT + 1;

static WINDOW: std::sync::atomic::AtomicPtr<std::ffi::c_void> =
    std::sync::atomic::AtomicPtr::new(std::ptr::null_mut());
static SWITCH_KEY: std::sync::atomic::AtomicU16 = std::sync::atomic::AtomicU16::new(0);
static CONFINE_KEY: std::sync::atomic::AtomicU16 = std::sync::atomic::AtomicU16::new(0);
static CONFINE_MODIFIERS: std::sync::Mutex<Vec<crate::Modifier>> =
    std::sync::Mutex::new(Vec::new());
static CONFINE_PRESSED: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

fn held(key: u16) -> bool {
    unsafe { windows_sys::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState(key as i32) < 0 }
}

unsafe extern "system" fn hook(
    code: i32,
    w_param: windows_sys::Win32::Foundation::WPARAM,
    l_param: windows_sys::Win32::Foundation::LPARAM,
) -> windows_sys::Win32::Foundation::LRESULT {
    let event = unsafe {
        &*(l_param as *const windows_sys::Win32::UI::WindowsAndMessaging::KBDLLHOOKSTRUCT)
    };
    // teleput types through SendInput, which marks its keys LLKHF_INJECTED: packages/teleput/src/client/windows.rs
    if code == windows_sys::Win32::UI::WindowsAndMessaging::HC_ACTION as i32 {
        let pressed = matches!(
            w_param as u32,
            windows_sys::Win32::UI::WindowsAndMessaging::WM_KEYDOWN
                | windows_sys::Win32::UI::WindowsAndMessaging::WM_SYSKEYDOWN
        );
        let key = event.vkCode as u16;
        let post = |message, w_param| unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::PostMessageW(
                WINDOW.load(std::sync::atomic::Ordering::SeqCst),
                message,
                w_param,
                0,
            );
        };
        let shift = held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_SHIFT) as usize;
        if crate::switch::windows::OPEN.load(std::sync::atomic::Ordering::SeqCst) {
            match key {
                _ if key == SWITCH_KEY.load(std::sync::atomic::Ordering::SeqCst) => {
                    if pressed {
                        post(crate::switch::windows::MESSAGE_ADVANCE, shift);
                    }
                    return 1;
                }
                windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_ESCAPE if pressed => {
                    post(crate::switch::windows::MESSAGE_CANCEL, 0);
                    return 1;
                }
                windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_LMENU
                | windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_RMENU
                    if !pressed =>
                {
                    post(crate::switch::windows::MESSAGE_COMMIT, 0);
                }
                _ => {}
            }
        } else if key == SWITCH_KEY.load(std::sync::atomic::Ordering::SeqCst)
            && pressed
            && held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_CONTROL)
            && held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_MENU)
        {
            crate::switch::windows::OPEN.store(true, std::sync::atomic::Ordering::SeqCst);
            post(crate::switch::windows::MESSAGE_OPEN, shift);
            return 1;
        }
        if key == CONFINE_KEY.load(std::sync::atomic::Ordering::SeqCst) {
            let modifiers =
                CONFINE_MODIFIERS
                    .lock()
                    .unwrap()
                    .iter()
                    .all(|modifier| match modifier {
                        crate::Modifier::Shift => {
                            held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_SHIFT)
                        }
                        crate::Modifier::Control => {
                            held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_CONTROL)
                        }
                        crate::Modifier::Alt => {
                            held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_MENU)
                        }
                        crate::Modifier::Super => {
                            held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_LWIN)
                                || held(windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_RWIN)
                        }
                    });
            if pressed && modifiers {
                // A held key repeats its key down.
                if !CONFINE_PRESSED.swap(true, std::sync::atomic::Ordering::SeqCst) {
                    post(MESSAGE_CONFINE, 0);
                }
                return 1;
            }
            if !pressed && CONFINE_PRESSED.swap(false, std::sync::atomic::Ordering::SeqCst) {
                return 1;
            }
        }
    }
    unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::CallNextHookEx(
            std::ptr::null_mut(),
            code,
            w_param,
            l_param,
        )
    }
}

pub fn run(args: crate::Args) -> crate::Return {
    let Some(window) = crate::switch::windows::create()? else {
        return Ok(());
    };
    WINDOW.store(window, std::sync::atomic::Ordering::SeqCst);
    SWITCH_KEY.store(args.switch_key, std::sync::atomic::Ordering::SeqCst);
    CONFINE_KEY.store(args.confine_key, std::sync::atomic::Ordering::SeqCst);
    *CONFINE_MODIFIERS.lock().unwrap() = args.confine_modifier;

    // The system silently removes a hook that does not return within LowLevelHooksTimeout, and a hook runs on the message loop of the thread installing it: https://learn.microsoft.com/en-us/windows/win32/winmsg/lowlevelkeyboardproc
    let (sender, receiver) = std::sync::mpsc::channel();
    std::thread::spawn(move || {
        let installed = unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::SetWindowsHookExW(
                windows_sys::Win32::UI::WindowsAndMessaging::WH_KEYBOARD_LL,
                Some(hook),
                windows_sys::Win32::System::LibraryLoader::GetModuleHandleW(std::ptr::null()),
                0,
            )
        };
        let failed = installed.is_null();
        let _ = sender.send(if failed {
            Err(std::io::Error::last_os_error())
        } else {
            Ok(())
        });
        if failed {
            return;
        }
        let mut message: windows_sys::Win32::UI::WindowsAndMessaging::MSG =
            unsafe { std::mem::zeroed() };
        while unsafe {
            windows_sys::Win32::UI::WindowsAndMessaging::GetMessageW(
                &mut message,
                std::ptr::null_mut(),
                0,
                0,
            )
        } > 0
        {}
    });
    receiver.recv()??;

    let mut confined = None;
    let mut message: windows_sys::Win32::UI::WindowsAndMessaging::MSG =
        unsafe { std::mem::zeroed() };
    while unsafe {
        windows_sys::Win32::UI::WindowsAndMessaging::GetMessageW(
            &mut message,
            std::ptr::null_mut(),
            0,
            0,
        )
    } > 0
    {
        match message.message {
            crate::switch::windows::MESSAGE_OPEN => {
                crate::switch::windows::open(window, message.wParam != 0)
            }
            crate::switch::windows::MESSAGE_ADVANCE => {
                crate::switch::windows::advance(window, message.wParam != 0)
            }
            crate::switch::windows::MESSAGE_CANCEL => crate::switch::windows::close(window, false),
            crate::switch::windows::MESSAGE_COMMIT => crate::switch::windows::close(window, true),
            MESSAGE_CONFINE => crate::confine::windows::run(window, &mut confined),
            windows_sys::Win32::UI::WindowsAndMessaging::WM_TIMER => {
                if let Some(rectangle) = &confined {
                    unsafe {
                        windows_sys::Win32::UI::WindowsAndMessaging::ClipCursor(rectangle);
                    }
                }
            }
            _ => unsafe {
                windows_sys::Win32::UI::WindowsAndMessaging::DispatchMessageW(&message);
            },
        }
    }
    Ok(())
}
