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

    /// Filename suffix to mask; each must be exactly 4 bytes and at most 8 may be given
    #[clap(short, long, default_values_t = [String::from(".csv"), String::from(".txt")])]
    sensitive_suffix: Vec<String>,
}

// Must match `.name` in src/bpf/mask.bpf.c (the mount's `root_bpf=`).
const BPF_NAME: &str = "mask_ops";

const PATH_DEPTH_MAX: usize = 64;

struct LookupEntry {
    parent: u64,
    name: std::ffi::OsString,
    lookups: u64,
}

// One nodeid can back several live fuse inodes, so an entry only goes once every lookup the
// kernel counted has been forgotten.
fn learn(
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    nodeid: u64,
    parent: u64,
    name: std::ffi::OsString,
) {
    match nodeid_to_entry.get_mut(&nodeid) {
        Some(entry) => {
            entry.parent = parent;
            entry.name = name;
            entry.lookups += 1;
        }
        None => {
            nodeid_to_entry.insert(
                nodeid,
                LookupEntry {
                    parent,
                    name,
                    lookups: 1,
                },
            );
        }
    }
}

fn forget(
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    nodeid: u64,
    nlookup: u64,
) {
    if let Some(entry) = nodeid_to_entry.get_mut(&nodeid) {
        entry.lookups = entry.lookups.saturating_sub(nlookup);
        if entry.lookups == 0 {
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
    let mut ambiguous: std::collections::HashSet<u64> = std::collections::HashSet::new();
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
                root_directory_fd,
                &mut nodeid_to_entry,
                &ambiguous,
            )?;
        } else if opcode == fuse::FUSE_MKDIR && is_postfilter {
            handle_mkdir_postfilter(&mut channel, &header, &mut nodeid_to_entry, &mut ambiguous)?;
        } else if opcode == fuse::FUSE_READ && !is_postfilter {
            handle_read(
                &mut channel,
                &header,
                &buffer[..length],
                root_directory_fd,
                &nodeid_to_entry,
            )?;
        } else if opcode == fuse::FUSE_CREATE && is_postfilter {
            handle_create_postfilter(
                &mut channel,
                &header,
                &buffer[..length],
                root_directory_fd,
                &mut nodeid_to_entry,
                &ambiguous,
            )?;
        } else if opcode == fuse::FUSE_FORGET {
            // Stale entries would otherwise become wrong components of a later path.
            if length >= fuse::IN_HEADER_SIZE + fuse::FORGET_IN_SIZE {
                let forget_in: fuse::FuseForgetIn =
                    fuse::read_struct(&buffer[fuse::IN_HEADER_SIZE..length]);
                forget(&mut nodeid_to_entry, header.nodeid, forget_in.nlookup);
            }
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
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    ambiguous: &std::collections::HashSet<u64>,
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
    let name = std::ffi::OsString::from_vec(name_bytes[..name_len].to_vec());

    let entry_start = name_start + name_len + 1;
    let entry_end = entry_start + fuse::ENTRY_OUT_SIZE;
    if entry_end > message.len() {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let mut entry_out: fuse::FuseEntryOut = fuse::read_struct(&message[entry_start..entry_end]);
    if entry_out.attr.ino == 0 || entry_out.attr.ino == fuse::FUSE_ROOT_ID {
        // Either would collide with a nodeid the daemon reads as "unknown" or "root".
        eprintln!(
            "masknas: unusable inode {} for {}",
            entry_out.attr.ino,
            name.to_string_lossy()
        );
        channel.reply_error(header.unique, -libc::EIO)?;
        return Ok(());
    }
    // Marking a name whose path cannot be rebuilt would leave its reads failing later, which is
    // worse than refusing here: nothing has been written, so this costs only the open.
    let parent = match backing_path(header.nodeid, nodeid_to_entry) {
        Some(parent) => parent,
        None => {
            eprintln!("masknas: lookup under unknown nodeid={}", header.nodeid);
            channel.reply_error(header.unique, -libc::EIO)?;
            return Ok(());
        }
    };
    // Under a directory made through the mount this nodeid names the parent, so the name could be
    // in either. The backing lookup already resolved it against the real parent, so compare. A
    // name absent from the parent never reaches here — the program filters those on error_in — so
    // a stat that fails outright is an anomaly rather than an answer, and refusing beats guessing.
    if ambiguous.contains(&header.nodeid) {
        let mut candidate = parent;
        candidate.push(&name);
        match fuse::inode_number(root_directory_fd, &candidate) {
            Ok(found) if found == entry_out.attr.ino => {}
            Ok(_) => {
                eprintln!(
                    "masknas: {} is not the {} this nodeid names",
                    candidate.display(),
                    name.to_string_lossy()
                );
                return channel.reply(header.unique, fuse::struct_bytes(&entry_out));
            }
            Err(error) => {
                eprintln!("masknas: cannot stat {}: {error}", candidate.display());
                channel.reply_error(header.unique, -libc::EIO)?;
                return Ok(());
            }
        }
    }

    // The backing path never fills nodeid, and the kernel adopts whatever this reply carries.
    // A directory takes the bare inode number: a child that inherits it must not inherit a mark.
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

// A directory made through the mount takes this parent's nodeid, so from here on a request
// naming the parent could belong to either. The daemon stops marking under it rather than
// resolving to the wrong file.
fn handle_mkdir_postfilter(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    ambiguous: &mut std::collections::HashSet<u64>,
) -> std::io::Result<()> {
    eprintln!(
        "masknas: nodeid={} is ambiguous from here on",
        header.nodeid
    );
    ambiguous.insert(header.nodeid);
    // The kernel took a lookup count on the parent to build the child, and will forget it later.
    if let Some(entry) = nodeid_to_entry.get_mut(&header.nodeid) {
        entry.lookups += 1;
    }
    channel.reply(header.unique, &[])
}

// FUSE_CREATE never runs a lookup, so without this the created file keeps nodeid 0 and its
// contents are served straight off the backing filesystem.
fn handle_create_postfilter(
    channel: &mut fuse::Channel,
    header: &fuse::FuseInHeader,
    message: &[u8],
    root_directory_fd: std::os::fd::RawFd,
    nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>,
    ambiguous: &std::collections::HashSet<u64>,
) -> std::io::Result<()> {
    let name_start = fuse::IN_HEADER_SIZE + fuse::CREATE_IN_SIZE;
    if name_start > message.len() {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let name_bytes = &message[name_start..];
    let name_len = match name_bytes.iter().position(|&byte| byte == 0) {
        Some(position) => position,
        None => {
            channel.reply_error(header.unique, -libc::EINVAL)?;
            return Ok(());
        }
    };
    let name = std::ffi::OsString::from_vec(name_bytes[..name_len].to_vec());

    let entry_start = name_start + name_len + 1;
    let open_start = entry_start + fuse::ENTRY_OUT_SIZE;
    if open_start + fuse::OPEN_OUT_SIZE > message.len() {
        channel.reply_error(header.unique, -libc::EINVAL)?;
        return Ok(());
    }
    let mut entry_out: fuse::FuseEntryOut = fuse::read_struct(&message[entry_start..open_start]);
    let open_out: fuse::FuseOpenOut = fuse::read_struct(&message[open_start..]);

    // The backing file already exists by the time this runs, so every path that cannot pin down
    // its inode replies with the reply untouched. That leaves nodeid at 0, which is byte for byte
    // what BPF_FUSE_CONTINUE would have produced, instead of failing an open whose file stays.
    let unmarked = |channel: &mut fuse::Channel| -> std::io::Result<()> {
        let mut reply = Vec::with_capacity(fuse::ENTRY_OUT_SIZE + fuse::OPEN_OUT_SIZE);
        reply.extend_from_slice(fuse::struct_bytes(&entry_out));
        reply.extend_from_slice(fuse::struct_bytes(&open_out));
        channel.reply(header.unique, &reply)
    };

    // Under a directory made through the mount this nodeid names the parent, so the path below
    // would name the wrong file. The file is already on disk, so leave it unmarked.
    if ambiguous.contains(&header.nodeid) {
        eprintln!(
            "masknas: create under nodeid={} which is ambiguous",
            header.nodeid
        );
        return unmarked(channel);
    }

    let parent = match backing_path(header.nodeid, nodeid_to_entry) {
        Some(parent) => parent,
        None => {
            eprintln!("masknas: create under unknown nodeid={}", header.nodeid);
            return unmarked(channel);
        }
    };
    // Catches a stored name that no longer resolves, or resolves to a different inode. The root's
    // nodeid is FUSE_ROOT_ID rather than a backing inode number, so there is nothing to compare
    // it against there.
    if header.nodeid != fuse::FUSE_ROOT_ID {
        match fuse::inode_number(root_directory_fd, &parent) {
            Ok(parent_nodeid) if parent_nodeid == header.nodeid => {}
            Ok(parent_nodeid) => {
                eprintln!(
                    "masknas: create under nodeid={} but {} is inode {parent_nodeid}",
                    header.nodeid,
                    parent.display()
                );
                return unmarked(channel);
            }
            Err(error) => {
                eprintln!("masknas: cannot stat parent {}: {error}", parent.display());
                return unmarked(channel);
            }
        }
    }

    let mut path = parent;
    path.push(&name);

    let nodeid = match fuse::inode_number(root_directory_fd, &path) {
        Ok(nodeid) if nodeid != 0 && nodeid != fuse::FUSE_ROOT_ID => nodeid,
        Ok(nodeid) => {
            eprintln!("masknas: unusable inode {nodeid} for {}", path.display());
            return unmarked(channel);
        }
        Err(error) => {
            eprintln!("masknas: cannot stat backing {}: {error}", path.display());
            return unmarked(channel);
        }
    };
    entry_out.nodeid = nodeid | fuse::SENSITIVE_TAG;
    println!(
        "masknas: create nodeid={} -> {}",
        entry_out.nodeid,
        path.display()
    );
    learn(nodeid_to_entry, entry_out.nodeid, header.nodeid, name);

    let mut reply = Vec::with_capacity(fuse::ENTRY_OUT_SIZE + fuse::OPEN_OUT_SIZE);
    reply.extend_from_slice(fuse::struct_bytes(&entry_out));
    reply.extend_from_slice(fuse::struct_bytes(&open_out));
    channel.reply(header.unique, &reply)
}

// The kernel batches forgets once it negotiates minor 16 or above, so this is the common path.
fn forget_batch(message: &[u8], nodeid_to_entry: &mut std::collections::HashMap<u64, LookupEntry>) {
    if message.len() < fuse::IN_HEADER_SIZE + fuse::BATCH_FORGET_IN_SIZE {
        return;
    }
    let batch: fuse::FuseBatchForgetIn = fuse::read_struct(&message[fuse::IN_HEADER_SIZE..]);

    let mut offset = fuse::IN_HEADER_SIZE + fuse::BATCH_FORGET_IN_SIZE;
    for _ in 0..batch.count {
        if offset + fuse::FORGET_ONE_SIZE > message.len() {
            break;
        }
        let one: fuse::FuseForgetOne = fuse::read_struct(&message[offset..]);
        forget(nodeid_to_entry, one.nodeid, one.nlookup);
        offset += fuse::FORGET_ONE_SIZE;
    }
}

// LOOKUP only ever reports one component, so the backing path is the chain up to the root.
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
