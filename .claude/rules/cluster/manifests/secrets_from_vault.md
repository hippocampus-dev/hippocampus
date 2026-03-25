---
paths:
  - "**/secrets_from_vault.yaml"
---

* When adding a new `secrets_from_vault.yaml`, also update `cluster/bin/initialize-vault.sh` and `cluster/bin/export.sh`
* Read the existing entries in both scripts to match the pattern exactly

## Required Updates in initialize-vault.sh

| Item | Pattern | Location in script |
|------|---------|-------------------|
| `vault kv put` | `kv/{app-name} KEY="${ENV_VAR}"` | Secret values section (before `vault auth enable kubernetes`) |
| `vault policy write` | Read policy for `kv/data/{app-name}` | Policy section (before `vault auth enable userpass`) |
| `vault write auth/kubernetes/role` | `{app-name}.{app-name}` role bound to argocd-repo-server | After corresponding policy |

## Required Updates in export.sh

| Item | Pattern | Location in script |
|------|---------|-------------------|
| Default value | `DEFAULT_{ENV_VAR}=""` | Default values section |
| `read` prompt | `read -e -i "${ENV_VAR:-$DEFAULT_{ENV_VAR}}" -p "..." {ENV_VAR}` | Interactive prompts section |
| `export` | `export {ENV_VAR}` | Export section |
| Persistence | `{ENV_VAR}="${ENV_VAR}"` | `cat <<EOS > ${ENV_FILE_PATH}` section |
