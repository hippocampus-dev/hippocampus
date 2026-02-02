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
   - If deployed as utility: `cluster/manifests/utilities/{app-name}`
   - If referenced by other apps: use those paths (comma-separated if multiple)
   - If image is referenced as env var in another controller's patch: use that patches directory (see "Image as Environment Variable" below)
   - If not deployed to Kubernetes: `""`

5. If the Dockerfile requires building from a parent directory:
   - Change `cd` path to `cd cluster/applications`
   - Add `-f {app-name}/Dockerfile` to the `docker buildx build` command

6. Do NOT add `if: github.repository_owner == 'kaidotio'` to the `publish` job:
   - Forks have GitHub Actions disabled by default
   - Even if enabled, forks push to their own GHCR namespace (not kaidotio's)
   - The condition adds noise without practical benefit

7. Add security scanning (see "Security Scanning" section below):
   - Add `id-token: write` to `permissions`
   - Add `scan` job after the build job

## Image as Environment Variable

When an image is referenced as an environment variable value in another controller's deployment (not in kustomization.yaml images section):

| Scenario | KUSTOMIZATION | Image Reference |
|----------|---------------|-----------------|
| Controller creates pods dynamically | `""` | `:main` tag |
| Static deployment with env var | patches directory | `@sha256:` digest |

### Controller-Managed Images (No Digest Pinning)

When a controller creates pods dynamically using an image passed via environment variable, use `:main` tag:

1. Set `KUSTOMIZATION: ""`
2. Reference image with `:main` tag in the deployment patch env var

The controller pulls the latest `:main` image when creating pods. Digest pinning is not needed because:
- Controller already manages pod lifecycle
- Pods are recreated when needed, picking up new image versions

Example: `github-actions-exporter` is referenced as `EXPORTER_IMAGE` env var with `:main` tag in `github-actions-exporter-controller` deployment.

### Static Deployment with Digest Pinning

When an image is referenced in a static deployment's env var but you want digest pinning, use `sed`:

1. Set `KUSTOMIZATION` to the patches directory containing the deployment patch
2. Replace the `kustomize edit set image` block with `sed`:

```bash
for target in "${targets[@]}"; do
  sed -i "s|${GHCR_IMAGE}@sha256:[a-f0-9]\{64\}|${GHCR_IMAGE}@${DIGEST}|g" ${target}/*.yaml
done
```

## Security Scanning

Add `id-token: write` to `permissions` and a `scan` job after the build job:

```yaml
permissions:
  contents: write
  packages: write
  pull-requests: write
  id-token: write  # Required for scan job

# ...

scan:
  if: github.repository_owner == 'kaidotio'
  needs: publish
  uses: ./.github/workflows/reusable_scan-image.yaml
  with:
    image: ghcr.io/${{ github.repository }}/{app-name}:${{ github.ref_name }}
  secrets: inherit
```

| Element | Value |
|---------|-------|
| `if: github.repository_owner == 'kaidotio'` | Required - scan calls `reusable_scan-image.yaml` which depends on self-hosted runner |
| `id-token: write` | Required permission for OIDC authentication |
| `needs` | Build job name (usually `publish`) |
| `image` | Full image reference with tag |
| `secrets` | Use `inherit` to pass GPG_PASSPHRASE |

The scan workflow uses Trivy to detect vulnerabilities and calls `.github/scripts/create-cve-issues.sh` to create one GitHub issue per CVE ID (deduplicated against open issues). Both `reusable_scan-image.yaml` and `99_scan-mirrored-images.yaml` share this script via sparse-checkout.

Note: Mirrored images (from `99_mirroring.yaml`) are scanned automatically by `99_scan-mirrored-images.yaml`, which triggers on `workflow_run` completion. Each mirroring job uploads an artifact when an image is actually pushed, and the scan workflow only scans images that were pushed (skipping unchanged images). No manual scan job configuration is needed for mirrored images.

## Workflow Naming Convention

| Pattern | Purpose |
|---------|---------|
| `00_{app}.yaml` | Application build workflows |
| `10_*.yaml` | Label handlers |
| `20_*.yaml` | Release/additional builds (armyknife, taurin, Cloudflare Workers) |
| `40_*.yaml` | Tests |
| `50_*.yaml` | Validation |
| `80_*.yaml` | Metrics/sync |
| `90_*.yaml` | Utilities (readme, crd, jsonnet) |
| `99_*.yaml` | Mirroring and snapshots |
| `reusable_*.yaml` | Reusable workflow templates |
