use clap::Parser;
use std::os::unix::ffi::OsStringExt;

mod bpf;
mod fuse;
mod pii;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(short, long, default_value = "/srv/nas-backing")]
    backing_directory: std::path::PathBuf,

    #[clap(short, long, default_value = "/srv/nas")]
    mount_directory: std::path::PathBuf,

    /// Filename suffix to mask; each must be at most 8 bytes and at most 8 may be given
    #[clap(short, long, default_values_t = [String::from(".csv"), String::from(".txt")])]
    sensitive_suffix: Vec<String>,
}

// Must match `.name` in src/bpf/mask.bpf.c (the mount's `root_bpf=`).
const BPF_NAME: &str = "mask_ops";

const PATH_DEPTH_MAX: usize = 64;

#[derive(Default)]
struct LookupEntry {
    parent: u64,
    name: std::ffi::OsString,
    lookups: u64,
}

fn learn(
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    nodeid: u64,
    parent: u64,
    name: std::ffi::OsString,
) {
    let entry = nodeid_to_entry.entry(nodeid).or_default();
    entry.parent = parent;
    entry.name = name;
    entry.lookups += 1;
}

fn forget(
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    nodeid: u64,
    nlookup: u64,
) {
    if let Some(entry) = nodeid_to_entry.get_mut(&nodeid) {
        entry.lookups = entry.lookups.saturating_sub(nlookup);
        if entry.lookups == 0 && nodeid & fuse::SENSITIVE_TAG != 0 {
            nodeid_to_entry.remove(&nodeid);
        }
    }
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

    let root_directory_fd = fuse::open_directory(backing_directory)?;
    let (root_device, _) = fuse::device_and_inode(root_directory_fd)?;

    let mut open_object = std::mem::MaybeUninit::uninit();
    let _attachment = bpf::attach(&mut open_object, sensitive_suffix)?;

    let mut channel = fuse::Channel::mount_and_init(mount_directory, BPF_NAME, root_directory_fd)?;
    println!(
        "masknas: mounted {} (backing {}), masking {} reads",
        mount_directory.display(),
        backing_directory.display(),
        sensitive_suffix.join("/")
    );

    let mut nodeid_to_entry: std::collections::HashMap<u64, LookupEntry> =
        std::collections::HashMap::new();
    let mut ambiguous: std::collections::HashSet<u64> = std::collections::HashSet::new();
    let mut buffer = vec![0u8; fuse::FUSE_BUFFER_SIZE];

    while !STOP.load(std::sync::atomic::Ordering::Relaxed) {
        let length = match channel.read_request(&mut buffer) {
            Ok(0) => break,
            Ok(length) => length,
            Err(error) if error.kind() == std::io::ErrorKind::Interrupted => continue,
            Err(error) if error.raw_os_error() == Some(libc::ENODEV) => break,
            Err(error) => return Err(error.into()),
        };
        let header: fuse::FuseInHeader = fuse::read_struct(&buffer[..length]);
        let opcode = header.opcode & fuse::FUSE_OPCODE_FILTER;
        let is_postfilter = header.opcode & fuse::FUSE_POSTFILTER != 0;

        if opcode == fuse::FUSE_LOOKUP && is_postfilter {
            handle_lookup_postfilter(
                &mut channel,
                &header,
                &buffer[..length],
                root_directory_fd,
                root_device,
                &mut nodeid_to_entry,
                &ambiguous,
            )?;
        } else if opcode == fuse::FUSE_MKDIR && is_postfilter {
            handle_mkdir_postfilter(&mut channel, &header, &mut ambiguous)?;
        } else if opcode == fuse::FUSE_READ && !is_postfilter {
            handle_read(
                &mut channel,
                &header,
                &buffer[..length],
                root_directory_fd,
                root_device,
                &nodeid_to_entry,
            )?;
        } else if opcode == fuse::FUSE_FORGET {
            let forget_in: fuse::FuseForgetIn =
                fuse::read_struct(&buffer[fuse::IN_HEADER_SIZE..length]);
            forget(&mut nodeid_to_entry, header.nodeid, forget_in.nlookup);
        } else if opcode == fuse::FUSE_BATCH_FORGET {
            forget_batch(&buffer[..length], &mut nodeid_to_entry);
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
    root_directory_fd: std::os::fd::RawFd,
    root_device: u64,
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    ambiguous: &std::collections::HashSet<u64>,
) -> std::io::Result<()> {
    let name_start = fuse::IN_HEADER_SIZE;
    let name_bytes = &message[name_start..];
    let name_len = name_bytes
        .iter()
        .position(|&byte| byte == 0)
        .expect("lookup name is NUL-terminated");
    let name = std::ffi::OsString::from_vec(name_bytes[..name_len].to_vec());

    let entry_start = name_start + name_len + 1;
    let entry_end = entry_start + fuse::ENTRY_OUT_SIZE;
    let mut entry_out: fuse::FuseEntryOut = fuse::read_struct(&message[entry_start..entry_end]);
    let parent = match backing_path(header.nodeid, nodeid_to_entry) {
        Some(parent) => parent,
        None => {
            eprintln!("masknas: lookup under unknown nodeid={}", header.nodeid);
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };
    if ambiguous.contains(&header.nodeid) {
        let mut candidate = parent;
        candidate.push(&name);
        if fuse::device_and_inode_at(root_directory_fd, &candidate).ok()
            != Some((root_device, entry_out.attr.ino))
        {
            eprintln!(
                "masknas: {} is not the inode this lookup reports, leaving it unmarked",
                candidate.display()
            );
            return channel.reply(header.unique, fuse::struct_bytes(&entry_out));
        }
    }

    entry_out.nodeid = if entry_out.attr.mode & libc::S_IFMT == libc::S_IFDIR {
        entry_out.attr.ino
    } else {
        entry_out.attr.ino | fuse::SENSITIVE_TAG
    };

    println!(
        "masknas: learn nodeid={} -> {} under {}",
        entry_out.nodeid,
        name.to_string_lossy(),
        header.nodeid
    );
    learn(nodeid_to_entry, entry_out.nodeid, header.nodeid, name);
    channel.reply(header.unique, fuse::struct_bytes(&entry_out))
}

fn handle_mkdir_postfilter(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    ambiguous: &mut std::collections::HashSet<u64>,
) -> std::io::Result<()> {
    eprintln!(
        "masknas: nodeid={} is ambiguous from here on",
        header.nodeid
    );
    ambiguous.insert(header.nodeid);
    channel.reply(header.unique, &[])
}

fn forget_batch(message: &[u8], nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>) {
    let batch: fuse::FuseBatchForgetIn = fuse::read_struct(&message[fuse::IN_HEADER_SIZE..]);

    let mut offset = fuse::IN_HEADER_SIZE + fuse::BATCH_FORGET_IN_SIZE;
    for _ in 0..batch.count {
        let one: fuse::FuseForgetOne = fuse::read_struct(&message[offset..]);
        forget(nodeid_to_entry, one.nodeid, one.nlookup);
        offset += fuse::FORGET_ONE_SIZE;
    }
}

fn backing_path(
    nodeid: u64,
    nodeid_to_entry: &std::collections::HashMap<u64, LookupEntry>,
) -> Option<std::path::PathBuf> {
    let mut components: Vec<&std::ffi::OsStr> = Vec::new();
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
    root_device: u64,
    nodeid_to_entry: &std::collections::HashMap<u64, LookupEntry>,
) -> std::io::Result<()> {
    let read_in: fuse::FuseReadIn = fuse::read_struct(&message[fuse::IN_HEADER_SIZE..]);

    let path = match backing_path(header.nodeid, nodeid_to_entry) {
        Some(path) => path,
        None => {
            eprintln!("masknas: read for unknown nodeid={}", header.nodeid);
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };

    let expected = (root_device, header.nodeid & !fuse::SENSITIVE_TAG);
    let mut contents = match fuse::read_backing_file(root_directory_fd, &path, expected) {
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

    let window = contents.get(read_in.offset as usize..).unwrap_or_default();
    channel.reply(
        header.unique,
        &window[..window.len().min(read_in.size as usize)],
    )
}

fn install_signal_handlers() {
    extern "C" fn on_signal(_signal: libc::c_int) {
        STOP.store(true, std::sync::atomic::Ordering::Relaxed);
    }
    unsafe {
        let mut action: libc::sigaction = std::mem::zeroed();
        action.sa_sigaction = on_signal as *const () as usize;
        action.sa_flags = 0;
        libc::sigemptyset(&mut action.sa_mask);
        libc::sigaction(libc::SIGINT, &action, std::ptr::null_mut());
        libc::sigaction(libc::SIGTERM, &action, std::ptr::null_mut());
    }
}
