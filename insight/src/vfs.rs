use plain::Plain;

mod skel {
    include!("vfs/skel.rs");
}
use skel::*;

const TASK_COMM_LEN: usize = 16;
const FILENAME_LEN: usize = 256;

#[repr(C)]
#[derive(Clone, Copy)]
struct RawVfsEvent {
    pid: u32,
    tgid: u32,
    uid: u32,
    _pad: u32,
    ts: u64,
    size: u64,
    ret: i64,
    op: u8,
    comm: [u8; TASK_COMM_LEN],
    filename: [u8; FILENAME_LEN],
}

impl Default for RawVfsEvent {
    fn default() -> Self {
        unsafe { std::mem::zeroed() }
    }
}

unsafe impl Plain for RawVfsEvent {}

#[derive(Debug, Clone, Copy)]
pub enum FileOp {
    Read,
    Write,
    Unknown(u8),
}

impl FileOp {
    pub fn label(&self) -> &'static str {
        match self {
            FileOp::Read => "READ",
            FileOp::Write => "WRITE",
            FileOp::Unknown(_) => "UNKWN",
        }
    }
}

#[derive(Debug, Clone)]
pub struct FileEvent {
    pub pid: u32,
    pub tgid: u32,
    pub uid: u32,
    pub timestamp_ns: u64,
    pub size: u64,
    pub ret: i64,
    pub op: FileOp,
    pub comm: String,
    pub filename: String,
}

impl FileEvent {
    pub fn is_error(&self) -> bool {
        self.ret < 0
    }

    pub fn error_name(&self) -> Option<&'static str> {
        if self.ret >= 0 {
            return None;
        }

        // Convert negative return value to errno
        let errno = (-self.ret) as i32;
        match errno {
            1 => Some("EPERM"),
            2 => Some("ENOENT"),
            3 => Some("ESRCH"),
            4 => Some("EINTR"),
            5 => Some("EIO"),
            6 => Some("ENXIO"),
            7 => Some("E2BIG"),
            8 => Some("ENOEXEC"),
            9 => Some("EBADF"),
            10 => Some("ECHILD"),
            11 => Some("EAGAIN"),
            12 => Some("ENOMEM"),
            13 => Some("EACCES"),
            14 => Some("EFAULT"),
            15 => Some("ENOTBLK"),
            16 => Some("EBUSY"),
            17 => Some("EEXIST"),
            18 => Some("EXDEV"),
            19 => Some("ENODEV"),
            20 => Some("ENOTDIR"),
            21 => Some("EISDIR"),
            22 => Some("EINVAL"),
            23 => Some("ENFILE"),
            24 => Some("EMFILE"),
            25 => Some("ENOTTY"),
            26 => Some("ETXTBSY"),
            27 => Some("EFBIG"),
            28 => Some("ENOSPC"),
            29 => Some("ESPIPE"),
            30 => Some("EROFS"),
            31 => Some("EMLINK"),
            32 => Some("EPIPE"),
            33 => Some("EDOM"),
            34 => Some("ERANGE"),
            _ => Some("UNKNOWN"),
        }
    }
}

impl From<RawVfsEvent> for FileEvent {
    fn from(event: RawVfsEvent) -> Self {
        FileEvent {
            pid: event.pid,
            tgid: event.tgid,
            uid: event.uid,
            timestamp_ns: event.ts,
            size: event.size,
            ret: event.ret,
            op: match event.op {
                0 => FileOp::Read,
                1 => FileOp::Write,
                other => FileOp::Unknown(other),
            },
            comm: bytes_to_string(&event.comm),
            filename: bytes_to_string(&event.filename),
        }
    }
}

impl FileEvent {
    pub fn timestamp_seconds(&self) -> f64 {
        self.timestamp_ns as f64 / 1_000_000_000.0
    }
}

fn bytes_to_string(bytes: &[u8]) -> String {
    let nul_pos = bytes.iter().position(|&b| b == 0).unwrap_or(bytes.len());
    String::from_utf8_lossy(&bytes[..nul_pos]).into_owned()
}

pub fn watch(
    args: crate::Args,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    shutdown: std::sync::Arc<std::sync::atomic::AtomicBool>,
    tx: std::sync::mpsc::Sender<crate::Event>,
) -> Result<(), error::Error> {
    let mut builder = VfsSkelBuilder::default();
    if args.debug {
        builder.obj_builder.debug(true);
    }

    let mut open = builder.open()?;
    open.rodata().tool_config.tgid = args.tgid.unwrap_or_default();

    let mut skel = open.load()?;
    skel.attach()?;

    let mut ringbuf_builder = libbpf_rs::RingBufferBuilder::new();
    ringbuf_builder.add(skel.maps().events(), move |data: &[u8]| -> i32 {
        let mut raw = RawVfsEvent::default();
        if let Err(err) = plain::copy_from_bytes(&mut raw, data) {
            eprintln!("failed to decode vfs event: {err:?}");
            return 0;
        }

        let event = FileEvent::from(raw);
        let vfs_event = crate::ResultEvent::Vfs {
            pid: event.pid,
            tgid: event.tgid,
            uid: event.uid,
            timestamp_ns: event.timestamp_ns,
            size: event.size,
            ret: event.ret,
            op: event.op,
            comm: event.comm,
            filename: event.filename,
        };

        tx.send(crate::Event::Result(vfs_event)).unwrap();
        0
    })?;

    let ringbuf = ringbuf_builder.build()?;

    loop {
        if shutdown.load(std::sync::atomic::Ordering::Relaxed) {
            break;
        }
        if stop.load(std::sync::atomic::Ordering::Relaxed) {
            std::thread::sleep(std::time::Duration::from_millis(100));
            continue;
        }
        ringbuf.poll(std::time::Duration::from_millis(100))?;
    }

    Ok(())
}
