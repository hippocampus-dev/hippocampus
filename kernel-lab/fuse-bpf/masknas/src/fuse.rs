use std::io::Read;
use std::io::Write;
use std::os::fd::AsRawFd;
use std::os::fd::FromRawFd;

pub const FUSE_OPCODE_FILTER: u32 = 0x0000_ffff;
pub const FUSE_POSTFILTER: u32 = 0x0002_0000;

pub const FUSE_LOOKUP: u32 = 1;
pub const FUSE_READ: u32 = 15;
pub const FUSE_INIT: u32 = 26;

const FUSE_KERNEL_VERSION: u32 = 7;

pub const FUSE_BUFFER_SIZE: usize = 1 << 20;

#[repr(C)]
#[derive(Clone, Copy, Debug)]
pub struct FuseInHeader {
    pub len: u32,
    pub opcode: u32,
    pub unique: u64,
    pub nodeid: u64,
    pub uid: u32,
    pub gid: u32,
    pub pid: u32,
    pub total_extlen: u16,
    pub padding: u16,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct FuseOutHeader {
    len: u32,
    error: i32,
    unique: u64,
}

#[repr(C)]
#[derive(Clone, Copy, Debug)]
pub struct FuseReadIn {
    pub fh: u64,
    pub offset: u64,
    pub size: u32,
    pub read_flags: u32,
    pub lock_owner: u64,
    pub flags: u32,
    pub padding: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct FuseInitIn {
    major: u32,
    minor: u32,
    max_readahead: u32,
    flags: u32,
}

#[repr(C)]
#[derive(Clone, Copy, Default)]
struct FuseInitOut {
    major: u32,
    minor: u32,
    max_readahead: u32,
    flags: u32,
    max_background: u16,
    congestion_threshold: u16,
    max_write: u32,
    time_gran: u32,
    max_pages: u16,
    map_alignment: u16,
    flags2: u32,
    unused: [u32; 7],
}

// buffer is from read(2), so read unaligned.
pub fn read_struct<T: Copy>(buffer: &[u8]) -> T {
    assert!(
        buffer.len() >= std::mem::size_of::<T>(),
        "fuse message shorter than {}",
        std::mem::size_of::<T>()
    );
    unsafe { (buffer.as_ptr() as *const T).read_unaligned() }
}

fn struct_bytes<T: Copy>(value: &T) -> &[u8] {
    unsafe { std::slice::from_raw_parts(value as *const T as *const u8, std::mem::size_of::<T>()) }
}

pub struct Channel {
    device: std::fs::File,
}

impl Channel {
    pub fn mount_and_init(
        mount_dir: &str,
        bpf_name: &str,
        root_dir_fd: std::os::fd::RawFd,
    ) -> std::io::Result<Self> {
        let device = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open("/dev/fuse")?;
        let device_fd = device.as_raw_fd();

        let options = format!(
            "fd={device_fd},user_id=0,group_id=0,rootmode=0040000,root_bpf={bpf_name},root_dir={root_dir_fd}"
        );
        // device stays owned: if mount_fuse fails, dropping it closes the fd.
        mount_fuse(mount_dir, &options)?;

        let mut channel = Self { device };
        channel.handshake()?;
        Ok(channel)
    }

    fn handshake(&mut self) -> std::io::Result<()> {
        let mut buffer = vec![0u8; FUSE_BUFFER_SIZE];
        let length = self.device.read(&mut buffer)?;
        let header: FuseInHeader = read_struct(&buffer[..length]);
        if header.opcode & FUSE_OPCODE_FILTER != FUSE_INIT {
            return Err(std::io::Error::other(format!(
                "expected FUSE_INIT first, got opcode {}",
                header.opcode
            )));
        }
        let init_in: FuseInitIn = read_struct(&buffer[std::mem::size_of::<FuseInHeader>()..length]);
        let init_out = FuseInitOut {
            major: FUSE_KERNEL_VERSION,
            minor: init_in.minor,
            max_readahead: 4096,
            flags: 0,
            max_write: 4096,
            time_gran: 1000,
            max_pages: 12,
            map_alignment: 4096,
            ..FuseInitOut::default()
        };
        self.reply(header.unique, struct_bytes(&init_out))
    }

    pub fn read_request(&mut self, buffer: &mut [u8]) -> std::io::Result<usize> {
        self.device.read(buffer)
    }

    pub fn reply(&mut self, unique: u64, payload: &[u8]) -> std::io::Result<()> {
        let header = FuseOutHeader {
            len: (std::mem::size_of::<FuseOutHeader>() + payload.len()) as u32,
            error: 0,
            unique,
        };
        let mut message = Vec::with_capacity(header.len as usize);
        message.extend_from_slice(struct_bytes(&header));
        message.extend_from_slice(payload);
        self.write_all(&message)
    }

    /// `error` is a negative errno, matching TESTFUSEOUTERROR.
    pub fn reply_error(&mut self, unique: u64, error: i32) -> std::io::Result<()> {
        let header = FuseOutHeader {
            len: std::mem::size_of::<FuseOutHeader>() as u32,
            error,
            unique,
        };
        self.write_all(struct_bytes(&header))
    }

    fn write_all(&mut self, message: &[u8]) -> std::io::Result<()> {
        // FUSE requires exactly one write(2) per reply.
        let written = self.device.write(message)?;
        if written != message.len() {
            return Err(std::io::Error::other(format!(
                "short fuse reply: wrote {written} of {}",
                message.len()
            )));
        }
        Ok(())
    }
}

fn mount_fuse(mount_dir: &str, options: &str) -> std::io::Result<()> {
    let source = std::ffi::CString::new("masknas").unwrap();
    let target = std::ffi::CString::new(mount_dir)?;
    let fstype = std::ffi::CString::new("fuse").unwrap();
    let data = std::ffi::CString::new(options)?;

    let result = unsafe {
        libc::mount(
            source.as_ptr() as *const libc::c_char,
            target.as_ptr() as *const libc::c_char,
            fstype.as_ptr() as *const libc::c_char,
            0 as libc::c_ulong,
            data.as_ptr() as *const libc::c_void,
        )
    };
    if result != 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}

pub fn unmount(mount_dir: &str) -> std::io::Result<()> {
    let target = std::ffi::CString::new(mount_dir)?;
    let result = unsafe { libc::umount2(target.as_ptr() as *const libc::c_char, libc::MNT_DETACH) };
    if result != 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}

pub fn read_backing_file(dir_fd: std::os::fd::RawFd, name: &str) -> std::io::Result<Vec<u8>> {
    let c_name = std::ffi::CString::new(name)
        .map_err(|_| std::io::Error::other("backing name has interior NUL"))?;
    let file_fd = unsafe {
        libc::openat(
            dir_fd,
            c_name.as_ptr() as *const libc::c_char,
            libc::O_RDONLY | libc::O_CLOEXEC,
        )
    };
    if file_fd < 0 {
        return Err(std::io::Error::last_os_error());
    }
    let mut file = unsafe { std::fs::File::from_raw_fd(file_fd) };
    let mut contents = Vec::new();
    file.read_to_end(&mut contents)?;
    Ok(contents)
}

pub fn open_directory(dir: &str) -> std::io::Result<std::os::fd::RawFd> {
    let c_dir = std::ffi::CString::new(dir)?;
    let fd = unsafe {
        libc::open(
            c_dir.as_ptr() as *const libc::c_char,
            libc::O_DIRECTORY | libc::O_RDONLY | libc::O_CLOEXEC,
        )
    };
    if fd < 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(fd)
}

pub const IN_HEADER_SIZE: usize = std::mem::size_of::<FuseInHeader>();
pub const READ_IN_SIZE: usize = std::mem::size_of::<FuseReadIn>();

// The wire structs must match the C ABI sizes.
const _: () = {
    assert!(IN_HEADER_SIZE == 40);
    assert!(std::mem::size_of::<FuseOutHeader>() == 16);
    assert!(READ_IN_SIZE == 40);
    assert!(std::mem::size_of::<FuseInitOut>() == 64);
};
