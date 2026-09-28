#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

if [ "$GITHUB_REPOSITORY" != "hippocampus-dev/hippocampus" ]; then
  exit 0
fi

IMAGE_PATH="${GITHUB_REPOSITORY}/mirror/${IMAGE}"
GHCR_IMAGE="ghcr.io/${IMAGE_PATH}"

# docker push reported a digest GHCR does not serve for registry.k8s.io/autoscaling/vpa-* on 2026-08-23, so the digest has to be read back from GHCR rather than taken from the push
resolve_digest() {
  curl -sSLo /dev/null --retry 5 -H @<(printf 'Authorization: Bearer %s\n' "$(echo "$GITHUB_TOKEN" | base64 -w0)") -H "Accept: application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json" "https://ghcr.io/v2/${IMAGE_PATH}/manifests/${TAG}" -D - | awk '/docker-content-digest/' | awk '/sha256:[a-f0-9]{64}/ { match($0, /sha256:[a-f0-9]{64}/); last=substr($0, RSTART, RLENGTH) } END { print last }'
}

set +e
DIGEST=$(resolve_digest)
set -e

if [ -z "$DIGEST" ]; then
  docker pull "${IMAGE}:${TAG}"
  docker tag "${IMAGE}:${TAG}" "${GHCR_IMAGE}:${TAG}"
  docker push "${GHCR_IMAGE}:${TAG}"
  DIGEST=$(resolve_digest)
  echo "pushed=true" >> "$GITHUB_OUTPUT"
  mkdir -p "${RUNNER_TEMP}/pushed"
  echo "${IMAGE}:${TAG}" > "${RUNNER_TEMP}/pushed/image.txt"
fi

if [ -n "$KUSTOMIZATION" ]; then
  if [ -z "$DIGEST" ]; then
    echo "no digest for ${GHCR_IMAGE}:${TAG}" >&2
    exit 1
  fi

  git config --global user.email "kaidotio@gmail.com"
  git config --global user.name "kaidotio"
  branch="${IMAGE//\//-}"
  git checkout -b "$branch"

  IFS_BAK=$IFS
  IFS=,
  targets=($KUSTOMIZATION)
  IFS=$IFS_BAK
  for target in "${targets[@]}"; do
    (
      cd "$target"
      kustomize edit set image "${IMAGE}=${GHCR_IMAGE}@${DIGEST}"
    )
  done

  git add "${targets[@]}"
  if git commit -m "Mirror $IMAGE"; then
    git push -f origin "$branch"
    if [ $(gh pr list --head "$branch" --json id | jq '. | length') -eq 0 ]; then
      gh pr create --title "Mirror $IMAGE" --body "${GHCR_IMAGE}@${DIGEST}"
    fi
  fi
fi
