# Invoking aquasecurity/trivy-action

How to configure `aquasecurity/trivy-action`, including running it several times in one job as `.github/workflows/reusable_scan-image.yaml` does to produce both a vulnerability report and a CycloneDX SBOM from one image.

## Step Placement

Run the invocations as consecutive steps of one job, even when each writes to a distinct `output` path.
The action clears and rewrites a hardcoded `./trivy_envs.txt` that its `entrypoint.sh` sources back, so concurrent invocations overwrite each other's `TRIVY_FORMAT`/`TRIVY_OUTPUT` and one of them silently writes no output file.
Nothing fails at the scan step itself; the failure surfaces far downstream as `No files were found with the provided path` from `actions/upload-artifact` or as a file guard in a later script.
Keep the first invocation unconditional, since every invocation after it depends on the install directory `setup-trivy` appended to `$GITHUB_PATH` and on the cache directory the first one populated.

## Cache Directory

Point the action at a cache location through its `cache-dir` input rather than `env: TRIVY_CACHE_DIR`.
The action's `action.yaml` always re-exports `TRIVY_CACHE_DIR` from its own `cache-dir` input (default `${{ github.workspace }}/.cache/trivy`) immediately before running, silently overriding any caller-set `env:` value.
Give every invocation the same `cache-dir` and add `cache: false` and `skip-setup-trivy: true` to every invocation after the first.
The action's internal `actions/cache` key is `cache-trivy-<date>` and never varies by directory, so distinct `cache-dir` values only register duplicate post-job saves under an identical key while each invocation still re-downloads the vulnerability DB and the trivy binary.

## Scanner Parity and Step Order

Declare the same `scanners` on every invocation and place the SBOM-format invocation (`format: cyclonedx`) first.
Trivy derives its blob cache key from the enabled analyzer set, and `--format cyclonedx` implicitly disables security scanning, so an invocation left at the default `scanners` computes a different key and silently re-pulls and re-analyzes every layer, observable only as `Missing diff ID in cache` under `--debug`.
Package file digests are recorded only for SBOM formats (`FileChecksum`) and that flag is not part of the cache key, so whichever invocation runs first decides whether digests are stored: with a non-SBOM invocation first, the cached entries carry no digests and the SBOM loses its component `hashes`.
Only the SBOM invocation's `scanners` is load-bearing, since a non-SBOM invocation already resolves to the same default set, but declare it on both so the parity is visible.
Repeat `severity` on every invocation as well: it is a report filter outside the cache key, and the action skips exporting `TRIVY_SEVERITY` when the input equals its default, so an omitted `severity` silently widens that invocation's report to every severity.

When switching an existing job to this ordering, delete the accumulated `cache-trivy-*` Actions caches once.
The action restores with `restore-keys: cache-trivy-`, so entries saved under the previous ordering keep being carried forward and the SBOM invocation reuses their digest-less blobs.

## Example

Copy from: `.github/workflows/reusable_scan-image.yaml`
