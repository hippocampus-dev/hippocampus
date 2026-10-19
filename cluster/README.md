# cluster

<!-- TOC -->
* [cluster](#cluster)
  * [Usage](#usage)
<!-- TOC -->

cluster is the Kubernetes infrastructure directory containing ArgoCD applications, secrets, and deployment orchestration.

## Usage

1. Modify `cluster/bin/export.sh` to export the secret as an environment variable
2. Modify `cluster/bin/initialize-vault.sh` to add the secret to the vault
3. Use the secret in `kind: SecretsFromVault`
4. Add the secret to the vault manually - `read` takes the GitHub token `cluster/bin/export.sh` prompts for, which `cluster/bin/initialize-vault.sh` registers as the `kaidotio` userpass password
    ```sh
    $ kubectl exec -it vault-0 -n vault -- sh
    / $ read -r GITHUB_TOKEN
    / $ vault login -method=userpass username=kaidotio password=$GITHUB_TOKEN
    ```
