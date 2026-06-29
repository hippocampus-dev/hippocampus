# github-actions-runner-controller

<!-- TOC -->
* [github-actions-runner-controller](#github-actions-runner-controller)
  * [Usage](#usage)
    * [Organization Runner](#organization-runner)
    * [GitHub Apps](#github-apps)
      * [Required Permissions](#required-permissions)
  * [Development](#development)
<!-- TOC -->

github-actions-runner-controller is a Kubernetes Custom Controller that runs a self-hosted runner of GitHub Actions.

## Usage

```sh
$ echo -n "<YOUR GITHUB TOKEN>" > examples/GITHUB_TOKEN
$ kubectl apply -k examples
```
The runner is based on an image that defined at `Runner` manifest.
Its image is rebuilt as an image for Runner using [GoogleContainerTools/kaniko](https://github.com/GoogleContainerTools/kaniko) by github-actions-runner-controller, and it is distributed via local docker registry.

```sh
$ cat examples/runner.yaml
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: example
spec:
  image: ubuntu:24.04
  owner: kaidotio
  repo: hippocampus
  tokenSecretKeyRef:
    name: credentials
    key: GITHUB_TOKEN

# This shows the image is pulling from the local docker registry
$ kubectl get pod -l app=example -o custom-columns=NAME:.metadata.name,IMAGE:.spec.containers[0].image
NAME                      IMAGE
example-6dd7c8974c-4sgjv  127.0.0.1:31994/f601e6d

# This shows the image is based on ubuntu:18.04
$ kubectl exec -it example-6dd7c8974c-4sgjv cat /etc/os-release
NAME="Ubuntu"
VERSION="18.04.4 LTS (Bionic Beaver)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 18.04.4 LTS"
VERSION_ID="18.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=bionic
UBUNTU_CODENAME=bionic
```

You can pass additional information to runner pod via `builderContainerSpec`, `runnerContainerSpec`, and `template`.

```yaml
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: example
spec:
  image: ubuntu:24.04
  owner: kaidotio
  repo: hippocampus
  tokenSecretKeyRef:
    name: credentials
    key: TOKEN
  builderContainerSpec:
    resource:
      requests:
        cpu: 1000m
  runnerContainerSpec:
    env:
      - name: FOO
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
      - name: BAR
        value: bar
  template:
    metadata:
      labels:
        sidecar.istio.io/inject: "false"
```

### Organization Runner

To register runners at the organization level, omit the `repo` field:

```yaml
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: org-runner
spec:
  image: ubuntu:24.04
  owner: kaidotio
  tokenSecretKeyRef:
    name: credentials
    key: GITHUB_TOKEN
```

The scope is automatically inferred from the `repo` field:
- If `repo` is set: repository-level runner
- If `repo` is omitted: organization-level runner

### GitHub Apps

You can use GitHub Apps to authenticate the runner.

```sh
kubectl create secret generic credentials --from-literal=github_app_id="<YOUR GITHUB APP ID>" --from-literal=github_app_installation_id="<YOUR GITHUB APP INSTALLATION ID>" --from-file=github_app_private_key="<PATH TO YOUR GITHUB APP PRIVATE KEY>"
cat <<EOF | kubectl apply -f -
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: github-apps-example
spec:
  image: ubuntu:24.04
  owner: kaidotio
  repo: hippocampus
  appSecretRef:
    name: credentials
EOF
```

#### Required Permissions

- Actions (read)
- Administration (read / write)
- Metadata (read)

## Development

```sh
$ make dev
```
