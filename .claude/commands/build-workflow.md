---
description: Create a new GitHub Actions build workflow
argument-hint: [APPLICATION_NAME]
---

Follow these steps to create a new GitHub Actions workflow.

1. Create a new GitHub Actions workflow file in `.github/workflows/00_$ARGUMENT.yaml`
2. Modify `paths` to trigger on source files:
   - Exclude Kubernetes-related directories like `manifests/` or `skaffold/` if they exist
   - Include shared dependencies if the application references files outside its directory
3. Modify `env.IMAGE` to match the application name
4. If `cluster/manifests/$ARGUMENT/base` exists, set `env.KUSTOMIZATION` to `cluster/manifests/$ARGUMENT/base`
5. If the Dockerfile requires building from a parent directory, use `-f $ARGUMENT/Dockerfile` in the build step
