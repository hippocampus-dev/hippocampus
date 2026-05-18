pub mod core;
pub mod cpu_usage;
pub mod http;
pub mod https;
pub mod mysql;
pub mod ui;
pub mod vfs;

#[derive(clap::Parser, Clone, Debug, Default)]
pub struct Args {
    #[clap(short, long)]
    pub tgid: Option<u32>,
    #[clap(short, long)]
    pub debug: bool,
    #[clap(long, default_value = "/usr/lib/libssl.so")]
    pub libssl_path: std::path::PathBuf,
    #[clap(long, default_value = "3306")]
    pub mysql_port: Option<u16>,
    /// Command to execute and monitor (remaining arguments after --)
    #[clap(last = true)]
    pub command: Vec<String>,
}

#[derive(Clone, Debug)]
pub enum ResultEvent {
    L7(String),
    TracePipe(String),
    Mysql {
        tgid: u32,
        pid: u32,
        uid: u32,
        fd: i32,
        direction: String,
        packet_length: u32,
        sequence_id: u8,
        parsed_command: Option<mysql_protocol_parser::Command>,
        parsed_response: Option<mysql_protocol_parser::QueryResponse>,
        handshake_info: Option<String>,
        data: Vec<u8>,
    },
    Vfs {
        pid: u32,
        tgid: u32,
        uid: u32,
        timestamp_ns: u64,
        size: u64,
        ret: i64,
        op: vfs::FileOp,
        comm: String,
        filename: String,
    },
}

#[derive(Clone, Debug)]
pub enum HistogramEvent {
    CPUUsage(std::collections::HashMap<String, crate::core::types::Histogram>),
}

#[derive(Clone, Debug)]
pub enum GaugeEvent {
    CPUUtilization(f64),
}

#[derive(Clone, Debug)]
pub enum Event {
    Input(termion::event::Event),
    Histogram(HistogramEvent),
    Gauge(GaugeEvent),
    Result(ResultEvent),
}

pub fn errno_to_name(errno: i64) -> &'static str {
    if errno >= 0 {
        return "OK";
    }

    let errno = (-errno) as i32;
    match errno {
        1 => "EPERM",
        2 => "ENOENT",
        3 => "ESRCH",
        4 => "EINTR",
        5 => "EIO",
        6 => "ENXIO",
        7 => "E2BIG",
        8 => "ENOEXEC",
        9 => "EBADF",
        10 => "ECHILD",
        11 => "EAGAIN",
        12 => "ENOMEM",
        13 => "EACCES",
        14 => "EFAULT",
        15 => "ENOTBLK",
        16 => "EBUSY",
        17 => "EEXIST",
        18 => "EXDEV",
        19 => "ENODEV",
        20 => "ENOTDIR",
        21 => "EISDIR",
        22 => "EINVAL",
        23 => "ENFILE",
        24 => "EMFILE",
        25 => "ENOTTY",
        26 => "ETXTBSY",
        27 => "EFBIG",
        28 => "ENOSPC",
        29 => "ESPIPE",
        30 => "EROFS",
        31 => "EMLINK",
        32 => "EPIPE",
        33 => "EDOM",
        34 => "ERANGE",
        _ => "UNKNOWN",
    }
}
