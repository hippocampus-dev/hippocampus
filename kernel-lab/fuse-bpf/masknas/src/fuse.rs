use std::io::Read;
use std::io::Write;
use std::os::fd::AsRawFd;
use std::os::fd::FromRawFd;
use std::os::unix::ffi::OsStrExt;

pub const FUSE_OPCODE_FILTER: u32 = 0x0000_ffff;
pub const FUSE_POSTFILTER: u32 = 0x0002_0000;

pub const FUSE_ROOT_ID: u64 = 1;

pub const FUSE_LOOKUP: u32 = 1;
pub const FUSE_FORGET: u32 = 2;
pub const FUSE_READ: u32 = 15;
pub const FUSE_INIT: u32 = 26;
pub const FUSE_BATCH_FORGET: u32 = 42;

const FUSE_KERNEL_VERSION: u32 = 7;
// The wire sizes asserted below are the ones this minor fixes; a newer kernel is answered with it.
const FUSE_KERNEL_MINOR_VERSION: u32 = 39;

pub const FUSE_BUFFER_SIZE: usize = 1 << 20;

pub const IN_HEADER_SIZE: usize = std::mem::size_of::<FuseInHeader>();
pub const READ_IN_SIZE: usize = std::mem::size_of::<FuseReadIn>();
pub const ENTRY_OUT_SIZE: usize = std::mem::size_of::<FuseEntryOut>();

#[repr(C)]
#[derive(Clone, Copy, Debug, Default)]
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

unsafe impl plain::Plain for FuseInHeader {}

#[repr(C)]
#[derive(Clone, Copy)]
struct FuseOutHeader {
    len: u32,
    error: i32,
    unique: u64,
}

unsafe impl plain::Plain for FuseOutHeader {}

#[repr(C)]
#[derive(Clone, Copy, Debug, Default)]
pub struct FuseReadIn {
    pub fh: u64,
    pub offset: u64,
    pub size: u32,
    pub read_flags: u32,
    pub lock_owner: u64,
    pub flags: u32,
    pub padding: u32,
}

unsafe impl plain::Plain for FuseReadIn {}

#[repr(C)]
#[derive(Clone, Copy, Default)]
pub struct FuseAttr {
    pub ino: u64,
    pub size: u64,
    pub blocks: u64,
    pub atime: u64,
    pub mtime: u64,
    pub ctime: u64,
    pub atimensec: u32,
    pub mtimensec: u32,
    pub ctimensec: u32,
    pub mode: u32,
    pub nlink: u32,
    pub uid: u32,
    pub gid: u32,
    pub rdev: u32,
    pub blksize: u32,
    pub flags: u32,
}

unsafe impl plain::Plain for FuseAttr {}

#[repr(C)]
#[derive(Clone, Copy, Default)]
pub struct FuseEntryOut {
    pub nodeid: u64,
    pub generation: u64,
    pub entry_valid: u64,
    pub attr_valid: u64,
    pub entry_valid_nsec: u32,
    pub attr_valid_nsec: u32,
    pub attr: FuseAttr,
}

unsafe impl plain::Plain for FuseEntryOut {}

#[repr(C)]
#[derive(Clone, Copy, Default)]
struct FuseInitIn {
    major: u32,
    minor: u32,
    max_readahead: u32,
    flags: u32,
}

unsafe impl plain::Plain for FuseInitIn {}

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

unsafe impl plain::Plain for FuseInitOut {}

// buffer is from read(2), so copy out rather than borrow it in place.
pub fn read_struct<T: plain::Plain + Default>(buffer: &[u8]) -> T {
    let mut value = T::default();
    plain::copy_from_bytes(&mut value, buffer).expect("Data buffer was too short");
    value
}

fn struct_bytes<T: plain::Plain>(value: &T) -> &[u8] {
    unsafe { plain::as_bytes(value) }
}

pub struct Channel {
    // Declared first so the mount goes away before the fd it was created from.
    _mount: MountGuard,
    device: std::fs::File,
}

impl Channel {
    pub fn mount_and_init(
        mount_directory: &std::path::Path,
        bpf_name: &str,
        root_directory_fd: std::os::fd::RawFd,
    ) -> std::io::Result<Self> {
        let device = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open("/dev/fuse")?;
        let device_fd = device.as_raw_fd();

        let options = format!(
            "fd={device_fd},user_id=0,group_id=0,rootmode=0040000,root_bpf={bpf_name},root_dir={root_directory_fd}"
        );
        // device stays owned: if mount_fuse fails, dropping it closes the fd.
        mount_fuse(mount_directory, &options)?;

        // Built before the handshake, so a handshake failure unmounts on the way out.
        let mut channel = Self {
            _mount: MountGuard {
                mount_directory: mount_directory.to_path_buf(),
            },
            device,
        };
        channel.handshake()?;
        Ok(channel)
    }

    fn handshake(&mut self) -> std::io::Result<()> {
        let mut buffer = vec![0u8; FUSE_BUFFER_SIZE];
        let length = self.device.read(&mut buffer)?;
        if length < IN_HEADER_SIZE + std::mem::size_of::<FuseInitIn>() {
            return Err(std::io::Error::other(format!(
                "short FUSE_INIT: {length} bytes"
            )));
        }
        let header: FuseInHeader = read_struct(&buffer[..length]);
        if header.opcode & FUSE_OPCODE_FILTER != FUSE_INIT {
            return Err(std::io::Error::other(format!(
                "expected FUSE_INIT first, got opcode {}",
                header.opcode
            )));
        }
        let init_in: FuseInitIn = read_struct(&buffer[IN_HEADER_SIZE..length]);
        if init_in.major != FUSE_KERNEL_VERSION {
            return Err(std::io::Error::other(format!(
                "unsupported FUSE major {}",
                init_in.major
            )));
        }
        let init_out = FuseInitOut {
            major: FUSE_KERNEL_VERSION,
            minor: init_in.minor.min(FUSE_KERNEL_MINOR_VERSION),
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

fn mount_fuse(mount_directory: &std::path::Path, options: &str) -> std::io::Result<()> {
    let source = std::ffi::CString::new("masknas").unwrap();
    let target = std::ffi::CString::new(mount_directory.as_os_str().as_bytes())?;
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

/// Unmounts on drop, so no error path leaves a mount whose `mask_ops` has already detached.
struct MountGuard {
    mount_directory: std::path::PathBuf,
}

impl Drop for MountGuard {
    fn drop(&mut self) {
        // EINVAL: already unmounted, e.g. the loop exited on ENODEV.
        if let Err(error) = unmount(&self.mount_directory)
            && error.raw_os_error() != Some(libc::EINVAL)
        {
            eprintln!(
                "masknas: cannot unmount {}: {error}",
                self.mount_directory.display()
            );
        }
    }
}

fn unmount(mount_directory: &std::path::Path) -> std::io::Result<()> {
    let target = std::ffi::CString::new(mount_directory.as_os_str().as_bytes())?;
    let result = unsafe { libc::umount2(target.as_ptr() as *const libc::c_char, libc::MNT_DETACH) };
    if result != 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}

pub fn read_backing_file(
    directory_fd: std::os::fd::RawFd,
    path: &std::path::Path,
) -> std::io::Result<Vec<u8>> {
    let c_path = std::ffi::CString::new(path.as_os_str().as_bytes())?;
    let file_fd = unsafe {
        libc::openat(
            directory_fd,
            c_path.as_ptr() as *const libc::c_char,
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

pub fn open_directory(directory: &std::path::Path) -> std::io::Result<std::os::fd::RawFd> {
    let c_directory = std::ffi::CString::new(directory.as_os_str().as_bytes())?;
    let fd = unsafe {
        libc::open(
            c_directory.as_ptr() as *const libc::c_char,
            libc::O_DIRECTORY | libc::O_RDONLY | libc::O_CLOEXEC,
        )
    };
    if fd < 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(fd)
}

// The wire structs must match the C ABI sizes.
const _: () = {
    assert!(IN_HEADER_SIZE == 40);
    assert!(std::mem::size_of::<FuseOutHeader>() == 16);
    assert!(READ_IN_SIZE == 40);
    assert!(ENTRY_OUT_SIZE == 128);
    assert!(std::mem::size_of::<FuseInitOut>() == 64);
};

// FuseInitIn is only the prefix of a 64-byte fuse_init_in; the rest goes unread.
const _: () = assert!(std::mem::size_of::<FuseInitIn>() == 16);
