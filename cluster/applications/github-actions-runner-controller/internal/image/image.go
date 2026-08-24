package image

import (
	"crypto/sha256"
	"fmt"

	garV1 "github-actions-runner-controller/api/v1"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RepositoryName(base string, binaryVersion string, runnerVersion string) string {
	// ScaleSetReconciler.reconcileBuildJob builds only where no job of that name exists yet.
	return fmt.Sprintf("%x", sha256.Sum256([]byte(dockerfile(base, binaryVersion, runnerVersion))))[:7]
}

func BuilderContainer(kanikoImage string, destination string, builderContainerSpec garV1.BuilderContainerSpec) v1.Container {
	return v1.Container{
		Name:            "kaniko",
		Image:           kanikoImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Args: []string{
			"--dockerfile=Dockerfile",
			"--context=dir:///workspace",
			"--cache=true",
			"--compressed-caching=false",
			fmt.Sprintf("--destination=%s", destination),
		},
		EnvFrom: builderContainerSpec.EnvFrom,
		Env:     builderContainerSpec.Env,
		VolumeMounts: append([]v1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace/Dockerfile",
				SubPath:   "Dockerfile",
				ReadOnly:  true,
			},
		}, builderContainerSpec.VolumeMounts...),
		Resources:                builderContainerSpec.Resources,
		TerminationMessagePath:   v1.TerminationMessagePathDefault,
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}
}

func WorkspaceConfigMap(name string, namespace string, base string, binaryVersion string, runnerVersion string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"Dockerfile": dockerfile(base, binaryVersion, runnerVersion),
		},
	}
}

func dockerfile(base string, binaryVersion string, runnerVersion string) string {
	return fmt.Sprintf(`
FROM %s
USER root
ENV DEBIAN_FRONTEND=noninteractive
RUN (command -v apt && apt update && apt install -y ca-certificates iputils-ping tar sudo git) || \
      (command -v apt-get && apt-get update && apt-get install -y --no-install-recommends ca-certificates iputils-ping tar sudo git) || \
      (command -v dnf && dnf install -y ca-certificates iputils tar sudo git) || \
      (command -v yum && yum install -y ca-certificates iputils tar sudo git) || \
      (command -v zypper && zypper install -n ca-certificates iputils tar sudo git-core) || \
      (echo "Unknown OS version" && exit 1)

RUN (command -v apt && apt update && apt install -y podman podman-docker uidmap fuse-overlayfs) || \
      (command -v apt-get && apt-get update && apt-get install -y --no-install-recommends podman podman-docker uidmap fuse-overlayfs) || \
      (command -v dnf && dnf install -y podman podman-docker shadow-utils fuse-overlayfs) || \
      (command -v yum && yum install -y podman podman-docker shadow-utils fuse-overlayfs) || \
      (command -v zypper && zypper install -n podman podman-docker shadow fuse-overlayfs) || \
      (echo "Podman installation failed" && exit 1)

ADD https://github.com/hippocampus-dev/hippocampus/releases/download/v%s/runner_%s_linux_amd64 /usr/local/bin/runner
RUN chmod +x /usr/local/bin/runner

RUN echo 'runner::60000:60000::/home/runner:/bin/sh' >> /etc/passwd
RUN echo 'runner::60000:' >> /etc/group
RUN mkdir /home/runner && chown runner:runner /home/runner

RUN echo "runner:!:0:0:99999:7:::" >> /etc/shadow
RUN echo "runner ALL=(ALL) NOPASSWD: ALL" | sudo EDITOR='tee -a' visudo

# https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
RUN echo "runner:1:59999" >> /etc/subuid
RUN echo "runner:60001:5535" >> /etc/subuid
RUN echo "runner:1:59999" >> /etc/subgid
RUN echo "runner:60001:5535" >> /etc/subgid

RUN mkdir -p /home/runner/.config/containers
# fuse-overlayfs needs a /dev/fuse the pod does not carry
RUN echo '[storage]' > /home/runner/.config/containers/storage.conf
RUN echo 'driver = "vfs"' >> /home/runner/.config/containers/storage.conf
# The default net.ipv4.ping_group_range lands on a /proc/sys the pod mounts read-only
RUN echo '[containers]' > /home/runner/.config/containers/containers.conf
RUN echo 'default_sysctls = []' >> /home/runner/.config/containers/containers.conf

WORKDIR /home/runner

RUN /usr/local/bin/runner --only-install --runner-version %s

RUN chown -R runner:runner /home/runner

USER 60000

ENTRYPOINT ["/usr/local/bin/runner"]
`, base, binaryVersion, binaryVersion, runnerVersion)
}
