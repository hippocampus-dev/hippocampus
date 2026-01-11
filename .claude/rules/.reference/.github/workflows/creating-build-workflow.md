# Creating Build Workflow

How to create a new GitHub Actions workflow for building applications.

## Workflow

1. Copy an existing workflow file (e.g., `.github/workflows/00_bakery.yaml`) to `.github/workflows/00_{app-name}.yaml`

2. Replace the following values based on `{app-name}`:
   - `name`: Set to `{app-name}`
   - `paths`: Change `cluster/applications/<old>/**` to `cluster/applications/{app-name}/**`
   - `env.IMAGE`: Set to `{app-name}`
   - `cache.key`: Change to `${{ runner.os }}-{app-name}-docker-${{ hashFiles('cluster/applications/{app-name}/Dockerfile') }}`
   - `cache.restore-keys`: Change to `${{ runner.os }}-{app-name}-docker-`
   - `cd` path in build step: Change to `cd cluster/applications/{app-name}`

3. Modify `paths` to trigger on source files:
   - Exclude Kubernetes-related directories like `manifests/` or `skaffold/` if they exist
   - Include shared dependencies if the application references files outside its directory

4. Set `env.KUSTOMIZATION` to the directory where the image digest should be updated:
   - If `cluster/applications/{app-name}/manifests` exists: use it
   - Otherwise if `cluster/manifests/{app-name}/base` exists: use it
   - If referenced by other apps: use those paths (comma-separated if multiple)
   - If not deployed to Kubernetes: `""`

5. If the Dockerfile requires building from a parent directory:
   - Change `cd` path to `cd cluster/applications`
   - Add `-f {app-name}/Dockerfile` to the `docker buildx build` command

## Workflow Naming Convention

| Pattern | Purpose |
|---------|---------|
| `00_{app}.yaml` | Application build workflows |
| `10_*.yaml` | Label handlers |
| `20_*.yaml` | Additional builds (armyknife, tauri) |
| `40_*.yaml` | Tests |
| `50_*.yaml` | Validation |
| `80_*.yaml` | Metrics/sync |
| `90_*.yaml` | Utilities (readme, crd, jsonnet) |
| `99_*.yaml` | Mirroring and snapshots |
| `reusable_*.yaml` | Reusable workflow templates |
