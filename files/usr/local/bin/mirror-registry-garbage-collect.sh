#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

CONFIGURATION="/etc/docker/registry/config.yml"
ROOTDIRECTORY="/var/lib/registry"
# The docker.io mirror serves compose builds rather than the cluster, so what the cluster deploys cannot judge it.
SERVICES=(mirror-registry-ghcrio)

cd /opt/hippocampus

referenced=$(
  {
    minikube kubectl -- get pods --all-namespaces -o jsonpath='{range .items[*]}{..imageID}{"\n"}{end}'
    minikube kubectl -- get deployments,statefulsets,daemonsets,cronjobs,jobs --all-namespaces -o jsonpath='{range .items[*]}{..image}{"\n"}{end}'
  } | tr ' ' '\n' | awk '/@sha256:/ { sub(/.*@/, ""); print }' | sort -u
)

if [ -z "$referenced" ]; then
  echo "no referenced digest found" >&2
  exit 1
fi

for service in "${SERVICES[@]}"; do
  docker compose exec -T "$service" find "$ROOTDIRECTORY" -path "*/_manifests/revisions/sha256/*" -name link \
    | awk -v referenced="$referenced" '
        BEGIN {
          split(referenced, digests, "\n")
          for (i in digests) {
            referenced_digests[digests[i]] = 1
          }
        }
        {
          sub(/\/link$/, "")
          digest = $0
          sub(/.*\//, "", digest)
          if (!(("sha256:" digest) in referenced_digests)) {
            print
          }
        }
      ' \
    | docker compose exec -T "$service" xargs -r rm -rf
  docker compose exec -T "$service" registry garbage-collect "$CONFIGURATION"
  docker compose restart "$service"
done
