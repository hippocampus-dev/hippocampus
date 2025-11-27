#!/usr/bin/env bash

set -e

SERVICE_ACCOUNT_NAME=argocd-repo-server
SERVICE_ACCOUNT_NAMESPACE=argocd

kubectl -n vault exec -i vault-0 -- sh -s <<EOS
export VAULT_TOKEN="\$(cat /data/rootToken)"

vault secrets enable -version=2 kv

vault kv put kv/oauth2-proxy OAUTH2_PROXY_CLIENT_SECRET="${OAUTH2_PROXY_OAUTH_CLIENT_SECRET}" OAUTH2_PROXY_COOKIE_SECRET="${OAUTH2_PROXY_COOKIE_SECRET}"
vault kv put kv/argocd-notifications-controller SLACK_BOT_TOKEN="${NOTIFICATION_SLACK_BOT_TOKEN}"
vault kv put kv/embedding-retrieval OPENAI_API_KEY="${OPENAI_API_KEY}"
vault kv put kv/cortex-api OPENAI_API_KEY="${OPENAI_API_KEY}" SLACK_BOT_TOKEN="${CORTEX_SLACK_BOT_TOKEN}" GITHUB_TOKEN="${GITHUB_TOKEN}" GOOGLE_CLIENT_SECRET="${CORTEX_GOOGLE_CLIENT_SECRET}" GOOGLE_PRE_ISSUED_REFRESH_TOKEN="${CORTEX_GOOGLE_PRE_ISSUED_REFRESH_TOKEN}"
vault kv put kv/cortex-bot SLACK_APP_TOKEN="${CORTEX_SLACK_APP_TOKEN}" SLACK_BOT_TOKEN="${CORTEX_SLACK_BOT_TOKEN}" SLACK_SIGNING_SECRET="${CORTEX_SLACK_SIGNING_SECRET}" OPENAI_API_KEY="${OPENAI_API_KEY}" GITHUB_TOKEN="${GITHUB_TOKEN}" GOOGLE_CLIENT_SECRET="${CORTEX_GOOGLE_CLIENT_SECRET}" GOOGLE_PRE_ISSUED_REFRESH_TOKEN="${CORTEX_GOOGLE_PRE_ISSUED_REFRESH_TOKEN}"
vault kv put kv/slack-bolt-proxy SLACK_APP_TOKEN="${CORTEX_SLACK_APP_TOKEN}" SLACK_BOT_TOKEN="${CORTEX_SLACK_BOT_TOKEN}" SLACK_SIGNING_SECRET="${CORTEX_SLACK_SIGNING_SECRET}"
vault kv put kv/translator SLACK_APP_TOKEN="${TRANSLATOR_SLACK_APP_TOKEN}" SLACK_BOT_TOKEN="${TRANSLATOR_SLACK_BOT_TOKEN}" SLACK_SIGNING_SECRET="${TRANSLATOR_SLACK_SIGNING_SECRET}" OPENAI_API_KEY="${OPENAI_API_KEY}"
vault kv put kv/jupyterhub CONFIGPROXY_AUTH_TOKEN="${JUPYTERHUB_CONFIGPROXY_AUTH_TOKEN}" JUPYTERHUB_COOKIE_SECRET="${JUPYTERHUB_COOKIE_SECRET}"
vault kv put kv/runner DOCKER_CONFIG_JSON="${DOCKER_CONFIG_JSON}"
vault kv put kv/slack-logger slack_user="${SLACK_LOGGER_TIDB_SLACK_USER_PASSWORD}"
vault kv put kv/github-actions-runner-controller GITHUB_APP_PRIVATE_KEY="${HIPPOCAMPUS_DEV_GITHUB_APP_PRIVATE_KEY}"
vault kv put kv/hippocampus-dev-github-token-server GITHUB_APP_PRIVATE_KEY="${HIPPOCAMPUS_DEV_GITHUB_APP_PRIVATE_KEY}"

vault auth enable kubernetes
vault write auth/kubernetes/config kubernetes_host=https://kubernetes.default.svc.cluster.local kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt

vault policy write oauth2-proxy - <<EOSS
path "kv/data/oauth2-proxy" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/oauth2-proxy.oauth2-proxy policies=oauth2-proxy bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write argocd-notifications-controller - <<EOSS
path "kv/data/argocd-notifications-controller" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/argocd-notifications-controller.argocd policies=argocd-notifications-controller bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write embedding-retrieval - <<EOSS
path "kv/data/embedding-retrieval" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/embedding-retrieval.embedding-retrieval policies=embedding-retrieval bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write cortex-api - <<EOSS
path "kv/data/cortex-api" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/cortex-api.cortex-api policies=cortex-api bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write cortex-bot - <<EOSS
path "kv/data/cortex-bot" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/cortex-bot.cortex-bot policies=cortex-bot bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write slack-bolt-proxy - <<EOSS
path "kv/data/slack-bolt-proxy" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/slack-bolt-proxy.cortex-bot policies=slack-bolt-proxy bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write translator - <<EOSS
path "kv/data/translator" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/translator.translator policies=translator bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write jupyterhub - <<EOSS
path "kv/data/jupyterhub" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/jupyterhub.jupyterhub policies=jupyterhub bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write runner - <<EOSS
path "kv/data/runner" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/runner.runner policies=runner bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write slack-logger - <<EOSS
path "kv/data/slack-logger" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/slack-logger.slack-logger policies=slack-logger bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write github-actions-runner-controller - <<EOSS
path "kv/data/github-actions-runner-controller" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/github-actions-runner-controller.github-actions-runner-controller policies=github-actions-runner-controller bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault policy write hippocampus-dev-github-token-server - <<EOSS
path "kv/data/hippocampus-dev-github-token-server" {
  capabilities = ["read"]
}
EOSS
vault write auth/kubernetes/role/hippocampus-dev-github-token-server.hippocampus-dev-github-token-server policies=hippocampus-dev-github-token-server bound_service_account_names=${SERVICE_ACCOUNT_NAME} bound_service_account_namespaces=${SERVICE_ACCOUNT_NAMESPACE} ttl=1h

vault auth enable userpass

vault policy write admin - <<EOSS
path "kv/*" {
  capabilities = ["create", "read", "update", "patch", "delete", "list"]
}

path "sys/policy" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "sys/policies/acl/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "auth/kubernetes/role/*" {
  capabilities = ["create", "read", "update", "patch", "delete", "list"]
}
EOSS
vault write auth/userpass/users/kaidotio password=${GITHUB_TOKEN} policies=admin

vault token revoke \$(cat /data/rootToken)
EOS
