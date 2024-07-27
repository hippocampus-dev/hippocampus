#!/usr/bin/env bash

set -eo pipefail

# /data is persistent volume
# $ mount | grep "on /data"
# /dev/vda1 on /data type ext4 (rw,relatime)
export PERSISTENT_DIRECTORY=/data

mkdir -p $PERSISTENT_DIRECTORY/bin

ETCD_VERSION=v3.4.25
curl -fsSL https://storage.googleapis.com/etcd/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz | tar zx --no-same-owner -C $PERSISTENT_DIRECTORY/bin/ etcd-${ETCD_VERSION}-linux-amd64/etcdctl --strip-components=1

mkdir -p $PERSISTENT_DIRECTORY/jupyterhub/persistent
mkdir -p $PERSISTENT_DIRECTORY/jupyterhub/shared
chown 1000:1000 $PERSISTENT_DIRECTORY/jupyterhub/shared

export CACHE_DIRECTORY=$PERSISTENT_DIRECTORY/cache

# minikube --container-runtime=cri-o still wipe images
# https://github.com/cri-o/cri-o/pull/6022
# https://github.com/kubernetes/minikube/blob/v1.32.0/deploy/iso/minikube-iso/package/crio-bin/crio.conf#L39-L47
mkdir -p $CACHE_DIRECTORY/var/run/crio
[ -e /var/run/crio/version ] && cp /var/run/crio/version $CACHE_DIRECTORY/var/run/crio/version
mkdir -p $CACHE_DIRECTORY/var/lib/crio
[ -e /var/lib/crio/version ] && cp /var/lib/crio/version $CACHE_DIRECTORY/var/lib/crio/version

mkdir -p $CACHE_DIRECTORY/etc/containerd/certs.d/docker.io
cat <<EOS > $CACHE_DIRECTORY/etc/containerd/certs.d/docker.io/hosts.toml
server = "https://docker.io"

[host."http://host.minikube.internal:5000"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOS

mkdir -p $CACHE_DIRECTORY/etc/containerd/certs.d/ghcr.io
cat <<EOS > $CACHE_DIRECTORY/etc/containerd/certs.d/ghcr.io/hosts.toml
server = "https://ghcr.io"

[host."http://host.minikube.internal:5002"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOS

mkdir -p $CACHE_DIRECTORY/etc/containers
cat <<EOS > $CACHE_DIRECTORY/etc/containers/registries.conf
unqualified-search-registries = ["docker.io"]

[[registry]]
prefix = "docker.io"
location = "docker.io"
  [[registry.mirror]]
  location = "host.minikube.internal:5000"
  insecure = true

[[registry]]
prefix = "ghcr.io"
location = "ghcr.io"
  [[registry.mirror]]
  location = "host.minikube.internal:5002"
  insecure = true
EOS

cat <<'EOS' > /var/lib/boot2docker/bootlocal.sh
#!/usr/bin/env bash

find $CACHE_DIRECTORY -type f | while IFS= read -r file; do
  target=${file#$CACHE_DIRECTORY}
  mkdir -p $(dirname $target)
  cp $file $target
done

# istio-proxy launches two inotify watches per pod.
# --extra-config=kubelet.max-pods=300
# https://elixir.bootlin.com/linux/latest/source/fs/notify/inotify/inotify_user.c#L869
sysctl -w fs.inotify.max_user_instances=1024
EOS

sed -ri 's|\$CACHE_DIRECTORY|'$CACHE_DIRECTORY'|g' /var/lib/boot2docker/bootlocal.sh

sh /var/lib/boot2docker/bootlocal.sh
