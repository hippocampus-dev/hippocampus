package isolation

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	Userns     = "userns"
	Privileged = "privileged"
)

const (
	containersStorageVolumeName = "containers-storage"
	// podman takes this from HOME, which the Dockerfile image.WorkspaceConfigMap renders fixes at /home/runner
	containersStorageDirectory = "/home/runner/.local/share/containers/storage"
)

func Capabilities(mode string) []v1.Capability {
	capabilities := []v1.Capability{
		"SETGID",
		"SETUID",
		"AUDIT_WRITE",
	}
	if mode == Userns {
		// newuidmap opens a uid_map the runner does not own, and its write to that uid_map fails where SYS_ADMIN is absent
		// buildah's copier chroots into the build context
		capabilities = append(capabilities, "SYS_ADMIN", "DAC_OVERRIDE", "SYS_CHROOT")
	}
	return capabilities
}

func ProcMount(mode string) *v1.ProcMountType {
	if mode != Userns {
		return nil
	}
	// crun fails to mount proc for a nested container while containerd's masked paths are in place
	return ptr.To(v1.UnmaskedProcMount)
}

func SeccompProfile(mode string) *v1.SeccompProfile {
	if mode == Privileged {
		return nil
	}
	if mode == Userns {
		// containerd's default profile carries no pivot_root, which crun needs to enter the rootfs it builds for a RUN step
		return &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeUnconfined,
		}
	}
	return &v1.SeccompProfile{
		Type: v1.SeccompProfileTypeRuntimeDefault,
	}
}

func HostUsers(mode string) *bool {
	if mode != Userns {
		return nil
	}
	// the capabilities userns adds hold against the pod's own user namespace rather than against the node
	return ptr.To(false)
}

func StorageVolumes(mode string) []v1.Volume {
	if mode != Userns {
		return nil
	}
	return []v1.Volume{
		{
			Name: containersStorageVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
}

func StorageVolumeMounts(mode string) []v1.VolumeMount {
	if mode != Userns {
		return nil
	}
	// overlay refuses an upperdir on the overlayfs a pod's own rootfs is, while this volume comes from the node's
	return []v1.VolumeMount{
		{
			Name:      containersStorageVolumeName,
			MountPath: containersStorageDirectory,
		},
	}
}

func StorageEnv(mode string) []v1.EnvVar {
	if mode != Userns {
		return nil
	}
	// the vfs the rendered storage.conf names copies every layer whole, and this overrides it for the volume above
	return []v1.EnvVar{
		{
			Name:  "STORAGE_DRIVER",
			Value: "overlay",
		},
	}
}
