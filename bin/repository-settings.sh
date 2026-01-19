#!/usr/bin/env bash

set -eo pipefail

function usage() {
  cat <<EOS
Usage:
   repository-settings.sh (up|down)
EOS
}

REPOSITORY="hippocampus-dev/hippocampus"

args=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Please declare required environment variables: GITHUB_TOKEN" 1>&2
  exit 1
fi

branches=("main")
required=()
repository_secrets=("CLAUDE_CODE_OAUTH_TOKEN")
environment_secrets=("ANDROID_KEYSTORE_BASE64" "ANDROID_KEYSTORE_PASSWORD" "ANDROID_TAURIM_KEY_PASSWORD" "CLOUDFLARE_API_TOKEN" "GPG_PASSPHRASE" "PUBLIC_REPOSITORY_PAT" "TAURI_SIGNING_PRIVATE_KEY" "TAURI_SIGNING_PRIVATE_KEY_PASSWORD")

function up {
  local CONFIRM=""
  read -e -p "Would you like to update GitHub repository settings? [y/N]: " CONFIRM
  [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ] && return

  local message=$(curl -fsSL -XPATCH -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}" -d '{}' | jq -r '.message')
  [ "$message" != "null" ] && echo "You don't have Admin role" 1>&2 && exit 1

  local default_branch=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}" | jq -r '.default_branch')
  [ "$default_branch" == "null" ] && echo "Please create a repository at first" 1>&2 && exit 1
  local default_sha=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/git/ref/heads/${default_branch}" 2>/dev/null | jq -r '.object.sha')
  if [ -n "$default_sha" ]; then
    for branch in "${branches[@]}"; do
      if [ "$(curl -fsSL -XGET -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/git/ref/heads/${branch} | jq -r '.message')" == "Not Found" ]; then
        curl -fsSL -XPOST -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/git/refs -d @- <<EOS || (echo "Failed to create ${branch} branch" 2>&1; exit 1)
{
  "ref": "refs/heads/${branch}",
  "sha": "${default_sha}"
}
EOS
        echo "Successfully created ${branch} branch"
      fi

      local protection=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/branches/${branch}/protection)
      local contexts=$(echo ${protection} | jq -r '.required_status_checks.contexts')
      [ "${contexts}" == "null" ] && contexts="[]"
      for context in "${required[@]}"; do
        if [ "$(echo ${contexts} | jq -r --arg context "${context}" '. | map(select(. == $context)) | length')" -eq 0 ]; then
          contexts=$(echo ${contexts} | jq -r --arg context "${context}" '.[. | length] = $context')
        fi
      done
      local require_code_owner_reviews="true"
      local required_approving_review_count=$(echo ${protection} | jq -r '.required_pull_request_reviews.required_approving_review_count')
      [ "${required_approving_review_count}" == "null" ] && required_approving_review_count=1
      local restrictions=$(echo ${protection} | jq '.restrictions')
      if [ "${restrictions}" != "null" ]; then
        require_code_owner_reviews="false"
        local users=$(echo ${protection} | jq '.restrictions.users')
        [ "${users}" == "null" ] && users="[]"
        local teams=$(echo ${protection} | jq '.restrictions.teams')
        [ "${teams}" == "null" ] && teams="[]"
        local apps=$(echo ${protection} | jq '.restrictions.apps')
        [ "${apps}" == "null" ] && apps="[]"
        restrictions=$(cat <<EOS
{
  "users": $(echo ${users} | jq '. | map(.login)'),
  "teams": $(echo ${teams} | jq '. | map(.slug)'),
  "apps": $(echo ${apps} | jq '. | map(.slug)')
}
EOS
)
      fi
      local body=$(cat <<EOS
{
  "required_status_checks": {
    "strict": true,
    "contexts": ${contexts}
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "require_code_owner_reviews": ${require_code_owner_reviews},
    "dismiss_stale_reviews": true,
    "required_approving_review_count": ${required_approving_review_count}
  },
  "restrictions": ${restrictions}
}
EOS
)
      curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/branches/${branch}/protection -d "${body}" || (echo "Failed to protect ${branch} branch" 2>&1; exit 1)
      echo "Successfully protected ${branch} branch"
    done
  fi

  local body=$(cat <<EOS
{
  "has_issues": true,
  "has_projects": false,
  "has_wiki": false,
  "has_discussions": false,
  "allow_merge_commit": true,
  "allow_squash_merge": false,
  "allow_rebase_merge": false,
  "allow_auto_merge": true,
  "delete_branch_on_merge": true
}
EOS
)
  curl -fsSL -XPATCH -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY} -d "${body}" || (echo "Failed to update repository settings" 2>&1; exit 1)
  echo "Successfully updated repository settings"

  curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/permissions/workflow -d @- <<EOS || (echo "Failed to update workflow permissions" 2>&1; exit 1)
{
  "can_approve_pull_request_reviews": true
}
EOS
  echo "Successfully updated workflow permissions"

  local public_key=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/secrets/public-key)
  for secret in "${repository_secrets[@]}"; do
    local encrypted_value=$(curl -fsSL -XPOST -H "Content-Type: application/json" https://libsodium-encryptor.minikube.127.0.0.1.nip.io -d "$(jq -n --arg key "$(echo ${public_key} | jq -r '.key')" --arg value "${!secret}" '{$key,$value}')")
    if [ -n "${encrypted_value}" ]; then
      curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/secrets/${secret} -d "$(jq -n --arg key_id "$(echo ${public_key} | jq -r '.key_id')" --arg encrypted_value "${encrypted_value}" '{$key_id,$encrypted_value}')" || (echo "Failed to update a secret ${secret}" 2>&1; exit 1)
      echo "Successfully updated a secret ${secret}"
    fi
  done

  curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/environments/deployment -d @- <<EOS || (echo "Failed to create an environment" 2>&1; exit 1)
{
  "deployment_branch_policy": {
     "protected_branches": true,
     "custom_branch_policies": false
  }
}
EOS
  echo "Successfully created an environment"

  local environments_public_key=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/environments/deployment/secrets/public-key)
  for secret in "${environment_secrets[@]}"; do
    local encrypted_value=$(curl -fsSL -XPOST -H "Content-Type: application/json" https://libsodium-encryptor.minikube.127.0.0.1.nip.io -d "$(jq -n --arg key "$(echo ${environments_public_key} | jq -r '.key')" --arg value "${!secret}" '{$key,$value}')")
    if [ -n "${encrypted_value}" ]; then
      curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/environments/deployment/secrets/${secret} -d "$(jq -n --arg key_id "$(echo ${environments_public_key} | jq -r '.key_id')" --arg encrypted_value "${encrypted_value}" '{$key_id,$encrypted_value}')" || (echo "Failed to update an environment secret ${secret}" 2>&1; exit 1)
      echo "Successfully updated an environment secret ${secret}"
    fi
  done
}

function down {
  local CONFIRM=""
  read -e -p "Would you like to initialize GitHub repository settings? [y/N]: " CONFIRM
  [ "${CONFIRM}" != "y" ] && [ "${CONFIRM}" != "Y" ] && return

  local message=$(curl -fsSL -XPATCH -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY} -d '{}' | jq -r '.message')
  [ "${message}" != "null" ] && echo "You don't have Admin role" 1>&2 && exit 1

  for branch in "${branches[@]}}"; do
    curl -fsSL -XDELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/branches/${branch}/protection || (echo "Failed to delete a branch protection of ${branch} branch" 2>&1; exit 1)
    echo "Successfully deleted a branch protection of ${branch} branch"
  done

  curl -fsSL -XPATCH -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" -H "Authorization: Bearer ${GITHUB}" https://api.github.com/repos/${REPOSITORY} -d @- <<EOS || (echo "Failed to update repository settings" 2>&1; exit 1)
{
  "has_issues": true,
  "has_projects": true,
  "has_wiki": true,
  "has_discussions": true,
  "allow_merge_commit": true,
  "allow_squash_merge": true,
  "allow_rebase_merge": true,
  "allow_auto_merge": false,
  "delete_branch_on_merge": false
}
EOS
  echo "Successfully initialize repository settings"

  curl -fsSL -XPUT -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/permissions/workflow -d @- <<EOS || (echo "Failed to update workflow permissions" 2>&1; exit 1)
{
  "can_approve_pull_request_reviews": false
}
EOS
  echo "Successfully initialize workflow permissions"

  for secret in "${repository_secrets[@]}"; do
    curl -fsSL -XDELETE -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" -H "Authorization: Bearer ${GITHUB}" https://api.github.com/repos/${REPOSITORY}/actions/secrets/${secret} || (echo "Failed to delete a secret ${secret}" 2>&1; exit 1)
  done

  curl -fsSL -XDELETE -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" -H "Authorization: Bearer ${GITHUB}" https://api.github.com/repos/${REPOSITORY}/environments/deployment || (echo "Failed to delete an environment" 2>&1; exit 1)

  for secret in "${environment_secrets[@]}"; do
    curl -fsSL -XDELETE -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" -H "Authorization: Bearer ${GITHUB}" https://api.github.com/repos/${REPOSITORY}/environments/deployment/secrets/${secret} || (echo "Failed to delete an environment secret ${secret}" 2>&1; exit 1)
  done
}

case "${args[0]:-}" in
  up)
    up
    ;;
  down)
    down
    ;;
  *)
    usage
    exit 1
    ;;
esac
