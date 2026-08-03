use clap::Parser;

mod bpf;
mod fuse;
mod pii;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(short, long, default_value = "/srv/nas-backing")]
    backing_directory: std::path::PathBuf,

    #[clap(short, long, default_value = "/srv/nas")]
    mount_directory: std::path::PathBuf,

    /// Filename suffix to mask; each must be exactly 4 bytes and at most 8 may be given
    #[clap(short, long, default_values_t = [String::from(".csv"), String::from(".txt")])]
    sensitive_suffix: Vec<String>,
}

// Must match `.name` in src/bpf/mask.bpf.c (the mount's `root_bpf=`).
const BPF_NAME: &str = "mask_ops";

const PATH_DEPTH_MAX: usize = 64;

struct LookupEntry {
    parent: u64,
    name: String,
}

static STOP: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    let args: Args = Args::parse();
    run(
        &args.backing_directory,
        &args.mount_directory,
        &args.sensitive_suffix,
    )
}

fn run(
    backing_directory: &std::path::Path,
    mount_directory: &std::path::Path,
    sensitive_suffix: &[String],
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    install_signal_handlers();

    // Kept open for the daemon's lifetime: backing root, and we openat() it.
    let root_directory_fd = fuse::open_directory(backing_directory)?;

    let mut open_object = std::mem::MaybeUninit::uninit();
    // Guard must outlive the mount, or mask_ops detaches.
    let _attachment = bpf::attach(&mut open_object, sensitive_suffix)?;

    // Declared after the attachment: the channel unmounts on drop, before mask_ops detaches.
    let mut channel = fuse::Channel::mount_and_init(mount_directory, BPF_NAME, root_directory_fd)?;
    println!(
        "masknas: mounted {} (backing {}), masking {} reads",
        mount_directory.display(),
        backing_directory.display(),
        sensitive_suffix.join("/")
    );

    let mut nodeid_to_entry: std::collections::HashMap<u64, LookupEntry> =
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
                &mut nodeid_to_entry,
            )?;
        } else if opcode == fuse::FUSE_READ && !is_postfilter {
            handle_read(
                &mut channel,
                &header,
                &buffer[..length],
                root_directory_fd,
                &nodeid_to_entry,
            )?;
        } else if opcode == fuse::FUSE_FORGET || opcode == fuse::FUSE_BATCH_FORGET {
        } else {
            eprintln!("masknas: unexpected opcode {} in userspace", header.opcode);
            channel.reply_error(header.unique, -libc::ENOSYS)?;
        }
    }

    Ok(())
}

fn handle_lookup_postfilter(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    message: &[u8],
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
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
    let entry_end = entry_start + fuse::ENTRY_OUT_SIZE;
    if entry_end > message.len() {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let entry = &message[entry_start..entry_end];
    let entry_out: fuse::FuseEntryOut = fuse::read_struct(entry);
    let nodeid = entry_out.nodeid;

    println!(
        "masknas: learn nodeid={nodeid} -> {name} under {}",
        header.nodeid
    );
    nodeid_to_entry.insert(
        nodeid,
        LookupEntry {
            parent: header.nodeid,
            name,
        },
    );
    channel.reply(header.unique, entry)
}

// LOOKUP only ever reports one component, so the backing path is the chain up to the root.
fn backing_path(
    nodeid: u64,
    nodeid_to_entry: &std::collections::HashMap<u64, LookupEntry>,
) -> Option<std::path::PathBuf> {
    let mut components: Vec<&str> = Vec::new();
    let mut current = nodeid;
    while current != fuse::FUSE_ROOT_ID {
        if components.len() == PATH_DEPTH_MAX {
            return None;
        }
        let entry = nodeid_to_entry.get(&current)?;
        components.push(&entry.name);
        current = entry.parent;
    }
    components.reverse();
    Some(components.iter().collect())
}

fn handle_read(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    message: &[u8],
    root_directory_fd: std::os::fd::RawFd,
    nodeid_to_entry: &std::collections::HashMap<u64, LookupEntry>,
) -> std::io::Result<()> {
    if message.len() < fuse::IN_HEADER_SIZE + fuse::READ_IN_SIZE {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let read_in: fuse::FuseReadIn = fuse::read_struct(&message[fuse::IN_HEADER_SIZE..]);

    let path = match backing_path(header.nodeid, nodeid_to_entry) {
        Some(path) => path,
        None => {
            eprintln!("masknas: read for unknown nodeid={}", header.nodeid);
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };

    let mut contents = match fuse::read_backing_file(root_directory_fd, &path) {
        Ok(contents) => contents,
        Err(error) => {
            eprintln!("masknas: cannot read backing {}: {error}", path.display());
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };

    let masked = pii::mask(&mut contents);
    println!(
        "masknas: read {} offset={} size={} ({masked} PII span(s) masked)",
        path.display(),
        read_in.offset,
        read_in.size
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
