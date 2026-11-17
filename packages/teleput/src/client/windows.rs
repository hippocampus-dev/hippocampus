// Linux input-event codes (linux/input-event-codes.h) and the SendInput flags and mouseData for their press and release.
const BUTTONS: [(u32, u32, u32, u16); 5] = [
    (
        0x110,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_LEFTDOWN,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_LEFTUP,
        0,
    ),
    (
        0x111,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_RIGHTDOWN,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_RIGHTUP,
        0,
    ),
    (
        0x112,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_MIDDLEDOWN,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_MIDDLEUP,
        0,
    ),
    (
        0x113,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_XDOWN,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_XUP,
        windows_sys::Win32::UI::WindowsAndMessaging::XBUTTON1,
    ),
    (
        0x114,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_XDOWN,
        windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_XUP,
        windows_sys::Win32::UI::WindowsAndMessaging::XBUTTON2,
    ),
];

// evdev keycodes from KEY_ESC to KEY_KPDOT equal their Scan 1 Make codes, and the rest come from the same column: https://learn.microsoft.com/en-us/windows/win32/inputdev/about-keyboard-input#scan-codes
const KEYS: [(u32, u16); 47] = [
    (86, 0x56),
    (87, 0x57),
    (88, 0x58),
    (89, 0x73),
    (92, 0x79),
    (93, 0x70),
    (94, 0x7B),
    (96, 0xE01C),
    (97, 0xE01D),
    (98, 0xE035),
    (99, 0xE037),
    (100, 0xE038),
    (102, 0xE047),
    (103, 0xE048),
    (104, 0xE049),
    (105, 0xE04B),
    (106, 0xE04D),
    (107, 0xE04F),
    (108, 0xE050),
    (109, 0xE051),
    (110, 0xE052),
    (111, 0xE053),
    (113, 0xE020),
    (114, 0xE02E),
    (115, 0xE030),
    (117, 0x59),
    (121, 0x7E),
    (124, 0x7D),
    (125, 0xE05B),
    (126, 0xE05C),
    (127, 0xE05D),
    (163, 0xE019),
    (164, 0xE022),
    (165, 0xE010),
    (166, 0xE024),
    (183, 0x64),
    (184, 0x65),
    (185, 0x66),
    (186, 0x67),
    (187, 0x68),
    (188, 0x69),
    (189, 0x6A),
    (190, 0x6B),
    (191, 0x6C),
    (192, 0x6D),
    (193, 0x6E),
    (194, 0x76),
];

const KEY_NUMLOCK: u32 = 69;
const KEY_PAUSE: u32 = 119;

pub async fn run(
    link: impl std::future::Future<Output = Result<super::Link, error::Error>>,
) -> crate::Return {
    unsafe {
        windows_sys::Win32::UI::HiDpi::SetProcessDpiAwarenessContext(
            windows_sys::Win32::UI::HiDpi::DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2,
        )
    };
    let mut injector = Injector {
        pressed_keys: std::collections::HashSet::new(),
        pressed_buttons: std::collections::HashSet::new(),
    };
    let mut announced = None;
    // WM_DISPLAYCHANGE goes only to windows: https://learn.microsoft.com/en-us/windows/win32/gdi/wm-displaychange
    let mut poll = tokio::time::interval(std::time::Duration::from_secs(1));
    // The server sends a held key once and leaves repeating it to the client (DETECTABLE_AUTO_REPEAT in server.rs).
    let repeat = tokio::time::sleep(std::time::Duration::ZERO);
    tokio::pin!(repeat);
    let mut repeating = None;

    let mut link = link.await?;
    // A console event nothing receives falls through to the default handler, which ends the process before Injector releases the keys it holds (tokio src/signal/windows/sys.rs).
    let mut interrupt = tokio::signal::windows::ctrl_c()?;
    let mut close = tokio::signal::windows::ctrl_close()?;
    let mut break_ = tokio::signal::windows::ctrl_break()?;
    loop {
        tokio::select! {
            _ = interrupt.recv() => return Ok(()),
            _ = close.recv() => return Ok(()),
            _ = break_.recv() => return Ok(()),
            _ = &mut repeat, if repeating.is_some() => {
                let (key, interval) = repeating.unwrap();
                keyboard(key, true);
                repeat.as_mut().reset(tokio::time::Instant::now() + interval);
            }
            _ = poll.tick() => {
                let mut rectangles: Vec<windows_sys::Win32::Foundation::RECT> = Vec::new();
                unsafe {
                    windows_sys::Win32::Graphics::Gdi::EnumDisplayMonitors(
                        std::ptr::null_mut(),
                        std::ptr::null(),
                        Some(collect),
                        &mut rectangles as *mut Vec<windows_sys::Win32::Foundation::RECT>
                            as windows_sys::Win32::Foundation::LPARAM,
                    )
                };
                let left = unsafe {
                    windows_sys::Win32::UI::WindowsAndMessaging::GetSystemMetrics(
                        windows_sys::Win32::UI::WindowsAndMessaging::SM_XVIRTUALSCREEN,
                    )
                };
                let top = unsafe {
                    windows_sys::Win32::UI::WindowsAndMessaging::GetSystemMetrics(
                        windows_sys::Win32::UI::WindowsAndMessaging::SM_YVIRTUALSCREEN,
                    )
                };
                let screens: Vec<_> = rectangles
                    .iter()
                    .map(|rectangle| crate::protocol::Message::Screen {
                        x: (rectangle.left - left) as u32,
                        y: (rectangle.top - top) as u32,
                        width: (rectangle.right - rectangle.left) as u32,
                        height: (rectangle.bottom - rectangle.top) as u32,
                    })
                    .collect();
                if announced.as_ref() != Some(&screens) {
                    link.announce(&screens).await?;
                    announced = Some(screens);
                }
            }
            messages = link.receive() => {
                for message in messages? {
                    match message {
                        crate::protocol::Message::Leave => {
                            repeating = None;
                            injector.release();
                        }
                        crate::protocol::Message::Motion { x, y } => {
                            let width = unsafe {
                                windows_sys::Win32::UI::WindowsAndMessaging::GetSystemMetrics(
                                    windows_sys::Win32::UI::WindowsAndMessaging::SM_CXVIRTUALSCREEN,
                                )
                            } as f32;
                            let height = unsafe {
                                windows_sys::Win32::UI::WindowsAndMessaging::GetSystemMetrics(
                                    windows_sys::Win32::UI::WindowsAndMessaging::SM_CYVIRTUALSCREEN,
                                )
                            } as f32;
                            mouse(
                                windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_MOVE
                                    | windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_ABSOLUTE
                                    | windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_VIRTUALDESK,
                                (x * 65536.0 / width).ceil() as i32,
                                (y * 65536.0 / height).ceil() as i32,
                                0,
                            );
                        }
                        crate::protocol::Message::Scroll { dx, dy } => {
                            // WHEEL_DELTA is 120 like a libei notch, but a positive wheel value rotates forward: https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-mouseinput
                            if dy != 0 {
                                mouse(
                                    windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_WHEEL,
                                    0,
                                    0,
                                    -dy as u32,
                                );
                            }
                            if dx != 0 {
                                mouse(
                                    windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEEVENTF_HWHEEL,
                                    0,
                                    0,
                                    dx as u32,
                                );
                            }
                        }
                        crate::protocol::Message::Button { button, pressed } => {
                            let Some(&(_, down, up, data)) =
                                BUTTONS.iter().find(|(code, ..)| *code == button)
                            else {
                                continue;
                            };
                            if pressed {
                                injector.pressed_buttons.insert(button);
                                mouse(down, 0, 0, data.into());
                            } else {
                                injector.pressed_buttons.remove(&button);
                                mouse(up, 0, 0, data.into());
                            }
                        }
                        crate::protocol::Message::Key { key, pressed } => {
                            if pressed {
                                injector.pressed_keys.insert(key);
                                let mut delay = 0u32;
                                let mut speed = 0u32;
                                unsafe {
                                    windows_sys::Win32::UI::WindowsAndMessaging::SystemParametersInfoW(
                                        windows_sys::Win32::UI::WindowsAndMessaging::SPI_GETKEYBOARDDELAY,
                                        0,
                                        &mut delay as *mut u32 as *mut std::ffi::c_void,
                                        0,
                                    );
                                    windows_sys::Win32::UI::WindowsAndMessaging::SystemParametersInfoW(
                                        windows_sys::Win32::UI::WindowsAndMessaging::SPI_GETKEYBOARDSPEED,
                                        0,
                                        &mut speed as *mut u32 as *mut std::ffi::c_void,
                                        0,
                                    );
                                };
                                // The delay runs from 0 (about 250 ms) to 3 (about 1 s) and the speed from 0 (about 2.5 per second) to 31 (about 30 per second): https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfow
                                repeating = Some((
                                    key,
                                    std::time::Duration::from_secs_f32(1.0 / (2.5 + speed as f32 * 27.5 / 31.0)),
                                ));
                                repeat.as_mut().reset(
                                    tokio::time::Instant::now()
                                        + std::time::Duration::from_millis(250 * (u64::from(delay) + 1)),
                                );
                            } else {
                                injector.pressed_keys.remove(&key);
                                if repeating.is_some_and(|(repeated, _)| repeated == key) {
                                    repeating = None;
                                }
                            }
                            keyboard(key, pressed);
                        }
                        _ => {}
                    }
                }
            }
        }
    }
}

unsafe extern "system" fn collect(
    _: windows_sys::Win32::Graphics::Gdi::HMONITOR,
    _: windows_sys::Win32::Graphics::Gdi::HDC,
    rectangle: *mut windows_sys::Win32::Foundation::RECT,
    data: windows_sys::Win32::Foundation::LPARAM,
) -> windows_sys::core::BOOL {
    let rectangles = unsafe { &mut *(data as *mut Vec<windows_sys::Win32::Foundation::RECT>) };
    rectangles.push(unsafe { *rectangle });
    windows_sys::Win32::Foundation::TRUE
}

fn mouse(flags: u32, dx: i32, dy: i32, data: u32) {
    send(windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT {
        r#type: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_MOUSE,
        Anonymous: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_0 {
            mi: windows_sys::Win32::UI::Input::KeyboardAndMouse::MOUSEINPUT {
                dx,
                dy,
                mouseData: data,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    });
}

fn keyboard(key: u32, pressed: bool) {
    let release = if pressed {
        0
    } else {
        windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYEVENTF_KEYUP
    };
    let (virtual_key, scan, flags) = if key == KEY_NUMLOCK {
        // Num Lock shares 0x45 with Pause and toggles through its virtual key: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-keybd_event
        (
            windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_NUMLOCK,
            0x45,
            windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYEVENTF_EXTENDEDKEY | release,
        )
    } else if key == KEY_PAUSE {
        // Pause makes the three bytes E1 1D 45, which wScan cannot carry.
        (
            windows_sys::Win32::UI::Input::KeyboardAndMouse::VK_PAUSE,
            0,
            release,
        )
    } else {
        let Some(scan) = KEYS
            .iter()
            .find(|(code, _)| *code == key)
            .map(|(_, scan)| *scan)
            .or_else(|| (1..=83).contains(&key).then_some(key as u16))
        else {
            return;
        };
        let extended = if scan >> 8 == 0xE0 {
            windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYEVENTF_EXTENDEDKEY
        } else {
            0
        };
        (
            0,
            scan & 0xFF,
            windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYEVENTF_SCANCODE
                | extended
                | release,
        )
    };
    send(windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT {
        r#type: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_KEYBOARD,
        Anonymous: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT_0 {
            ki: windows_sys::Win32::UI::Input::KeyboardAndMouse::KEYBDINPUT {
                wVk: virtual_key,
                wScan: scan,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    });
}

fn send(input: windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT) {
    unsafe {
        windows_sys::Win32::UI::Input::KeyboardAndMouse::SendInput(
            1,
            &input,
            std::mem::size_of::<windows_sys::Win32::UI::Input::KeyboardAndMouse::INPUT>() as i32,
        )
    };
}

struct Injector {
    pressed_keys: std::collections::HashSet<u32>,
    pressed_buttons: std::collections::HashSet<u32>,
}

impl Injector {
    fn release(&mut self) {
        for key in std::mem::take(&mut self.pressed_keys) {
            keyboard(key, false);
        }
        for button in std::mem::take(&mut self.pressed_buttons) {
            if let Some(&(_, _, up, data)) = BUTTONS.iter().find(|(code, ..)| *code == button) {
                mouse(up, 0, 0, data.into());
            }
        }
    }
}

impl Drop for Injector {
    fn drop(&mut self) {
        self.release();
    }
}
