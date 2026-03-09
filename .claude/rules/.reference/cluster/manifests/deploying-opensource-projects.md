# Deploying External Projects

Workflow for deploying third-party GitHub projects to the cluster.

## When to Use

- Deploying open-source projects from GitHub
- Adding third-party tools/services to the cluster

## Workflow

1. Open the GitHub URL to understand the project type
2. Determine workload type from the table in `cluster/manifests.md`
3. Copy manifests from the Example in the appropriate workload reference
4. Mirror Docker images via `.github/workflows/99_mirroring.yaml`

## Image Mirroring

External images must be mirrored to avoid rate limits and ensure availability:

1. Add image to `.github/workflows/99_mirroring.yaml`
2. Use mirrored image path in kustomization.yaml
3. Pin by digest, not tag
