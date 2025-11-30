#!/usr/bin/env bash

set -e

if [ "${GITHUB_REPOSITORY}" != "hippocampus-dev/hippocampus" ]; then
  exit 0
fi

IMAGE_PATH=${GITHUB_REPOSITORY}/mirror/${IMAGE}
GHCR_IMAGE=ghcr.io/${IMAGE_PATH}

set +e
DIGEST=$(curl -fsSL -H "Authorization: Bearer $(echo ${GITHUB_TOKEN} | base64 -w0)" https://ghcr.io/v2/${IMAGE_PATH}/manifests/${TAG} -o /dev/null -D - | grep docker-content-digest | grep -Eo 'sha256:[a-z0-9]{64}')
set -e

if [ -z "${DIGEST}" ]; then
  docker pull ${IMAGE}:${TAG}
  docker tag ${IMAGE}:${TAG} ${GHCR_IMAGE}:${TAG}
  DIGEST=$(docker push ${GHCR_IMAGE}:${TAG} | grep -Eo 'sha256:[a-z0-9]{64}')
fi

if [ -n "${KUSTOMIZATION}" ]; then
  git config --global user.email "kaidotio@gmail.com"
  git config --global user.name "kaidotio"
  branch=${IMAGE}
  git checkout -b ${branch}

  IFS_BAK=${IFS}
  IFS=,
  targets=(${KUSTOMIZATION})
  IFS=${IFS_BAK}
  for target in "${targets[@]}"; do
    (
      cd ${target}
      kustomize edit set image ${IMAGE}=${GHCR_IMAGE}@${DIGEST}
    )
  done

  git add ${targets[*]}
  if git commit -m "Mirror ${IMAGE}"; then
    git push -f origin ${branch}
    if [ $(gh pr list --head ${branch} --json id | jq '. | length') -eq 0 ]; then
      gh pr create --title "Mirror ${IMAGE}" --body "${GHCR_IMAGE}@${DIGEST}"
    fi
  fi
fi
