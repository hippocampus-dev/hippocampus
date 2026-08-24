package isolation

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	Userns     = "userns"
	Privileged = "privileged"
)

func Capabilities(mode string) []v1.Capability {
	capabilities := []v1.Capability{
		"SETGID",
		"SETUID",
		"AUDIT_WRITE",
	}
	if mode == Userns {
		// https://github.com/containerd/containerd/blob/v2.2.4/contrib/seccomp/seccomp_default.go#L629
		// newuidmap opens a uid_map the runner does not own
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

func HostUsers(mode string, hostUsers *bool) *bool {
	if mode != Userns {
		return hostUsers
	}
	return ptr.To(false)
}
