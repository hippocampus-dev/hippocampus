mod skel {
    include!("bpf/skel.rs");
}

use libbpf_rs::skel::OpenSkel;
use libbpf_rs::skel::SkelBuilder;

const SUFFIX_MAX: usize = 8;
const SUFFIX_LEN: usize = 4;

pub struct Attachment<'obj> {
    _skel: skel::MaskSkel<'obj>,
    _link: libbpf_rs::Link,
}

pub fn attach<'obj>(
    open_object: &'obj mut std::mem::MaybeUninit<libbpf_rs::OpenObject>,
    suffixes: &[String],
) -> Result<Attachment<'obj>, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let suffixes_len = suffixes.len();
    if suffixes_len == 0 {
        return Err("No sensitive suffixes given".into());
    }
    if suffixes_len > SUFFIX_MAX {
        return Err("Too many sensitive suffixes".into());
    }
    let mut suffixes_array: [[u8; SUFFIX_LEN]; SUFFIX_MAX] = [[0; SUFFIX_LEN]; SUFFIX_MAX];
    for (i, suffix) in suffixes.iter().enumerate() {
        if suffix.len() != SUFFIX_LEN {
            return Err(format!("Sensitive suffix must be {SUFFIX_LEN} bytes: {suffix}").into());
        }
        suffixes_array[i].copy_from_slice(suffix.as_bytes());
    }

    let builder = skel::MaskSkelBuilder::default();
    let open = builder.open(open_object)?;
    open.maps.rodata_data.tool_config.suffixes = suffixes_array;
    open.maps.rodata_data.tool_config.suffixes_len = suffixes_len as u32;

    let mut load = open.load()?;
    let link = load.maps.mask_ops.attach_struct_ops()?;
    Ok(Attachment {
        _skel: load,
        _link: link,
    })
}
