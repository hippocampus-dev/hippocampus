#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

if [ "$GITHUB_REPOSITORY" != "hippocampus-dev/hippocampus" ]; then
  exit 0
fi

readonly IMAGE_PATH="${GITHUB_REPOSITORY}/mirror/${IMAGE}"
readonly GHCR_IMAGE="ghcr.io/${IMAGE_PATH}"

# docker push reported a digest GHCR does not serve for registry.k8s.io/autoscaling/vpa-* on 2026-08-23, so the digest has to be read back from GHCR rather than taken from the push
resolve_digest() {
  curl -sSLo /dev/null --retry 5 -H @<(printf 'Authorization: Bearer %s\n' "$(echo "$GITHUB_TOKEN" | base64 -w0)") -H "Accept: application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json" "https://ghcr.io/v2/${IMAGE_PATH}/manifests/${TAG}" -D - | awk '/docker-content-digest/' | awk '/sha256:[a-f0-9]{64}/ { match($0, /sha256:[a-f0-9]{64}/); last=substr($0, RSTART, RLENGTH) } END { print last }'
}

set +e
digest=$(resolve_digest)
set -e

if [ -z "$digest" ]; then
  docker pull "${IMAGE}:${TAG}"
  docker tag "${IMAGE}:${TAG}" "${GHCR_IMAGE}:${TAG}"
  docker push "${GHCR_IMAGE}:${TAG}"
  digest=$(resolve_digest)
  echo "pushed=true" >> "$GITHUB_OUTPUT"
  mkdir -p "${RUNNER_TEMP}/pushed"
  echo "${IMAGE}:${TAG}" > "${RUNNER_TEMP}/pushed/image.txt"
fi

if [ -n "$KUSTOMIZATION" ]; then
  if [ -z "$digest" ]; then
    echo "no digest for ${GHCR_IMAGE}:${TAG}" 1>&2
    exit 1
  fi

  git config --global user.email "kaidotio@gmail.com"
  git config --global user.name "kaidotio"
  branch="${IMAGE//\//-}"
  git checkout -b "$branch"

  IFS=, read -r -a targets <<< "$KUSTOMIZATION"
  for target in "${targets[@]}"; do
    (
      cd "$target"
      kustomize edit set image "${IMAGE}=${GHCR_IMAGE}@${digest}"
    )
  done

  git add "${targets[@]}"
  if git commit -m "Mirror $IMAGE"; then
    git push -f origin "$branch"
    pull_requests=$(gh pr list --head "$branch" --json id | jq 'length')
    if [ "$pull_requests" -eq 0 ]; then
      gh pr create --title "Mirror $IMAGE" --body "${GHCR_IMAGE}@${digest}"
    fi
  fi
fi
