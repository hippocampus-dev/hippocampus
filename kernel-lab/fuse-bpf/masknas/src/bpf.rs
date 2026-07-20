mod skel {
    include!("bpf/skel.rs");
}

use libbpf_rs::skel::OpenSkel;
use libbpf_rs::skel::SkelBuilder;

// Holds the skeleton and link alive; dropping it detaches mask_ops.
pub struct Attachment {
    _skel: skel::MaskSkel<'static>,
    _link: libbpf_rs::Link,
}

pub fn attach() -> Result<Attachment, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let builder = skel::MaskSkelBuilder::default();
    // Leaked so the skeleton borrows it for 'static (lives until process exit).
    let open_object = Box::leak(Box::new(std::mem::MaybeUninit::uninit()));
    let open = builder.open(open_object)?;
    let mut load = open.load()?;
    let link = load.maps.mask_ops.attach_struct_ops()?;
    Ok(Attachment {
        _skel: load,
        _link: link,
    })
}
