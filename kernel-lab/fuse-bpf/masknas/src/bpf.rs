mod skel {
    include!("bpf/skel.rs");
}

use libbpf_rs::skel::OpenSkel;
use libbpf_rs::skel::SkelBuilder;

const SUFFIX_MAX: usize = 8;
const SUFFIX_LEN_MAX: usize = 8;

pub struct Attachment<'obj> {
    _skel: skel::MaskSkel<'obj>,
    _link: libbpf_rs::Link,
}

pub fn attach<'obj>(
    open_object: &'obj mut std::mem::MaybeUninit<libbpf_rs::OpenObject>,
    suffixes: &[String],
) -> Result<Attachment<'obj>, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let suffixes_len = suffixes.len();
    if suffixes_len > SUFFIX_MAX {
        return Err("Too many sensitive suffixes".into());
    }
    let mut suffixes_array: [[u8; SUFFIX_LEN_MAX]; SUFFIX_MAX] = [[0; SUFFIX_LEN_MAX]; SUFFIX_MAX];
    let mut suffix_sizes: [u8; SUFFIX_MAX] = [0; SUFFIX_MAX];
    for (i, suffix) in suffixes.iter().enumerate() {
        let bytes = suffix.as_bytes();
        if bytes.len() > SUFFIX_LEN_MAX {
            return Err(format!(
                "Sensitive suffix must be at most {SUFFIX_LEN_MAX} bytes: {suffix}"
            )
            .into());
        }
        suffixes_array[i][SUFFIX_LEN_MAX - bytes.len()..].copy_from_slice(bytes);
        suffix_sizes[i] = bytes.len() as u8;
    }

    let builder = skel::MaskSkelBuilder::default();
    let open = builder.open(open_object)?;
    open.maps.rodata_data.tool_config.suffixes = suffixes_array;
    open.maps.rodata_data.tool_config.suffix_sizes = suffix_sizes;
    open.maps.rodata_data.tool_config.suffixes_len = suffixes_len as u32;

    let mut load = open.load()?;
    let link = load.maps.mask_ops.attach_struct_ops()?;
    Ok(Attachment {
        _skel: load,
        _link: link,
    })
}
