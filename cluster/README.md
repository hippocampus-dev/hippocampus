# cluster

<!-- TOC -->
* [cluster](#cluster)
  * [Usage](#usage)
<!-- TOC -->

cluster is the Kubernetes infrastructure directory containing ArgoCD applications, secrets, and deployment orchestration.

## Usage

1. Modify `cluster/bin/export-secrets.sh` to export the secret as an environment variable
2. Modify `cluster/setup-vault.sh` to add the secret to the vault
3. Use the secret in `kind: SecretsFromVault`
4. Add the secret to the vault manually
    ```sh
    $ kubectl exec -it vault-0 -n vault -- sh -c "GITHUB_TOKEN=$GITHUB_TOKEN sh"
    / $ vault login -method=userpass username=kaidotio password=$GITHUB_TOKEN
    ```
