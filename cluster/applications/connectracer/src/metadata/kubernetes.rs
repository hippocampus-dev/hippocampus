use std::io::BufRead;

#[derive(Clone, Debug)]
pub struct Metadata {
    container_id: String,
}

pub fn from_pid(pid: i32) -> Option<Metadata> {
    let var = std::env::var("PROCFS_PATH");
    let path = if let Ok(ref path) = var {
        std::path::Path::new(path)
    } else {
        std::path::Path::new("/proc")
    };
    let cgroup = path.join(pid.to_string()).join("cgroup");

    if let Ok(file) = std::fs::File::open(cgroup) {
        let mut reader = std::io::BufReader::new(file);
        let mut buffer = String::new();
        let _ = reader.read_line(&mut buffer);
        return buffer
            .trim_end()
            .split(':')
            .last()
            .and_then(extract_container_id)
            .map(|container_id| Metadata { container_id });
    }
    None
}

impl From<Metadata> for Vec<opentelemetry::KeyValue> {
    fn from(metadata: Metadata) -> Self {
        vec![opentelemetry::KeyValue::new(
            "container_id",
            metadata.container_id,
        )]
    }
}

enum CgroupDriver {
    Cgroupfs,
    Systemd,
}

fn detect_cgroup_driver<T: AsRef<str>>(cgroup_path: T) -> CgroupDriver {
    if cgroup_path.as_ref().starts_with("/kubepods.slice") {
        // https://github.com/kubernetes/kubernetes/blob/v1.26.1/pkg/kubelet/cm/cgroup_manager_linux.go#L82
        CgroupDriver::Systemd
    } else {
        // https://github.com/kubernetes/kubernetes/blob/v1.26.1/pkg/kubelet/cm/cgroup_manager_linux.go#L111
        CgroupDriver::Cgroupfs
    }
}

fn extract_container_id<T: AsRef<str>>(cgroup_path: T) -> Option<String> {
    // https://github.com/kubernetes/kubernetes/blob/v1.26.1/pkg/kubelet/cm/node_container_manager_linux.go#L40
    if !cgroup_path.as_ref().starts_with("/kubepods") {
        return None;
    }

    match detect_cgroup_driver(&cgroup_path) {
        // https://github.com/cri-o/cri-o/blob/v1.26.1/internal/config/cgmgr/cgroupfs.go#L65
        CgroupDriver::Cgroupfs => cgroup_path
            .as_ref()
            .split('/')
            .last()
            .map(|s| s.to_string()),
        // https://github.com/cri-o/cri-o/blob/v1.26.1/internal/config/cgmgr/systemd.go#L80
        CgroupDriver::Systemd => cgroup_path
            .as_ref()
            .split('/')
            .last()
            .and_then(|unit| unit.trim_end_matches(".scope").split('-').last())
            .map(|s| s.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cgroupfs() {
        assert_eq!(
            extract_container_id(
                "/kubepods/burstable/pod2f8e9660-4c9b-4815-8572-42045b6833b1/1f83cc611a94780a1289daff8a18175f6a1b42a962fb0227b594ccd7f942326a"
            ),
            Some("1f83cc611a94780a1289daff8a18175f6a1b42a962fb0227b594ccd7f942326a".to_string())
        );
    }

    #[test]
    fn systemd() {
        assert_eq!(
            extract_container_id(
                "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod9e056778_2beb_4f2b_9f33_eb0ece4e7eb2.slice/docker-acf7bef1aa7111b6936f63d5d46acb402996dbc62f01a720527b6a48a3748d55.scope"
            ),
            Some("acf7bef1aa7111b6936f63d5d46acb402996dbc62f01a720527b6a48a3748d55".to_string())
        );
    }
}
