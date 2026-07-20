mod bpf;
mod fuse;
mod pii;

// Must match `.name` in src/bpf/mask.bpf.c (the mount's `root_bpf=`).
const BPF_NAME: &str = "mask_ops";

const FUSE_ENTRY_OUT_SIZE: usize = 128;

const FUSE_FORGET: u32 = 2;
const FUSE_BATCH_FORGET: u32 = 42;

static STOP: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let arguments: Vec<String> = std::env::args().collect();
    if arguments.len() != 3 {
        return Err(format!("usage: {} <backing_dir> <mount_dir>", arguments[0]).into());
    }
    run(&arguments[1], &arguments[2])
}

fn run(
    backing_dir: &str,
    mount_dir: &str,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    install_signal_handlers();

    // Kept open for the daemon's lifetime: backing root, and we openat() it.
    let root_dir_fd = fuse::open_directory(backing_dir)?;

    // Guard must outlive the mount, or mask_ops detaches.
    let _attachment = bpf::attach()?;

    let mut channel = fuse::Channel::mount_and_init(mount_dir, BPF_NAME, root_dir_fd)?;
    println!("masknas: mounted {mount_dir} (backing {backing_dir}), masking .csv/.txt reads");

    let mut nodeid_to_name: std::collections::HashMap<u64, String> =
        std::collections::HashMap::new();
    let mut buffer = vec![0u8; fuse::FUSE_BUFFER_SIZE];

    while !STOP.load(std::sync::atomic::Ordering::Relaxed) {
        let length = match channel.read_request(&mut buffer) {
            Ok(0) => break,
            Ok(length) => length,
            Err(error) if error.raw_os_error() == Some(libc::EINTR) => continue,
            // ENODEV: the filesystem was unmounted out from under us.
            Err(error) if error.raw_os_error() == Some(libc::ENODEV) => break,
            Err(error) => return Err(error.into()),
        };
        if length < fuse::IN_HEADER_SIZE {
            continue;
        }

        let header: fuse::FuseInHeader = fuse::read_struct(&buffer[..length]);
        let opcode = header.opcode & fuse::FUSE_OPCODE_FILTER;
        let is_postfilter = header.opcode & fuse::FUSE_POSTFILTER != 0;

        if opcode == fuse::FUSE_LOOKUP && is_postfilter {
            handle_lookup_postfilter(
                &mut channel,
                &header,
                &buffer[..length],
                &mut nodeid_to_name,
            )?;
        } else if opcode == fuse::FUSE_READ && !is_postfilter {
            handle_read(
                &mut channel,
                &header,
                &buffer[..length],
                root_dir_fd,
                &nodeid_to_name,
            )?;
        } else if opcode == FUSE_FORGET || opcode == FUSE_BATCH_FORGET {
        } else {
            eprintln!("masknas: unexpected opcode {} in userspace", header.opcode);
            channel.reply_error(header.unique, -libc::ENOSYS)?;
        }
    }

    let _ = fuse::unmount(mount_dir);
    Ok(())
}

fn handle_lookup_postfilter(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    message: &[u8],
    nodeid_to_name: &mut std::collections::HashMap<u64, String>,
) -> std::io::Result<()> {
    let name_start = fuse::IN_HEADER_SIZE;
    let name_bytes = &message[name_start..];
    let name_len = match name_bytes.iter().position(|&byte| byte == 0) {
        Some(position) => position,
        None => {
            channel.reply_error(header.unique, -libc::EINVAL)?;
            return Ok(());
        }
    };
    let name = String::from_utf8_lossy(&name_bytes[..name_len]).into_owned();

    let entry_start = name_start + name_len + 1;
    let entry_end = entry_start + FUSE_ENTRY_OUT_SIZE;
    if entry_end > message.len() {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let entry = &message[entry_start..entry_end];
    let nodeid = u64::from_ne_bytes(entry[..8].try_into().unwrap());

    nodeid_to_name.insert(nodeid, name.clone());
    println!("masknas: learn nodeid={nodeid} -> {name}");
    channel.reply(header.unique, entry)
}

fn handle_read(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    message: &[u8],
    root_dir_fd: std::os::fd::RawFd,
    nodeid_to_name: &std::collections::HashMap<u64, String>,
) -> std::io::Result<()> {
    if message.len() < fuse::IN_HEADER_SIZE + fuse::READ_IN_SIZE {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let read_in: fuse::FuseReadIn = fuse::read_struct(&message[fuse::IN_HEADER_SIZE..]);

    let name = match nodeid_to_name.get(&header.nodeid) {
        Some(name) => name,
        None => {
            eprintln!("masknas: read for unknown nodeid={}", header.nodeid);
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };

    let mut contents = match fuse::read_backing_file(root_dir_fd, name) {
        Ok(contents) => contents,
        Err(error) => {
            eprintln!("masknas: cannot read backing {name}: {error}");
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };

    let masked = pii::mask(&mut contents);
    println!(
        "masknas: read {name} offset={} size={} ({masked} PII span(s) masked)",
        read_in.offset, read_in.size
    );

    let offset = read_in.offset as usize;
    let window = if offset >= contents.len() {
        &[][..]
    } else {
        let end = offset
            .saturating_add(read_in.size as usize)
            .min(contents.len());
        &contents[offset..end]
    };
    channel.reply(header.unique, window)
}

fn install_signal_handlers() {
    extern "C" fn on_signal(_signal: libc::c_int) {
        STOP.store(true, std::sync::atomic::Ordering::Relaxed);
    }
    // sa_flags=0 (no SA_RESTART) so the signal interrupts the blocking read()
    // and the loop observes STOP; glibc's signal() would set SA_RESTART.
    unsafe {
        let mut action: libc::sigaction = std::mem::zeroed();
        action.sa_sigaction = on_signal as *const () as usize;
        action.sa_flags = 0;
        libc::sigemptyset(&mut action.sa_mask);
        libc::sigaction(libc::SIGINT, &action, std::ptr::null_mut());
        libc::sigaction(libc::SIGTERM, &action, std::ptr::null_mut());
    }
}
