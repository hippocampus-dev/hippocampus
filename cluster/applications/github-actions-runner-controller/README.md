# github-actions-runner-controller

github-actions-runner-controller is Kubernetes Custom Controller that runs self-hosted runner of GitHub Actions.

## Usage

```sh
$ echo -n "<YOUR GITHUB TOKEN>" > examples/GITHUB_TOKEN
$ kubectl apply -k examples
```
The runner is based on an image that defined at `Runner` manifest.
Its image is rebuilt as an image for Runner using [GoogleContainerTools/kaniko](https://github.com/GoogleContainerTools/kaniko) by github-actions-runner-controller, and it is distributed via local docker registry.

```shell
$ cat examples/runner.yaml
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: example
spec:
  image: ubuntu:22.04
  repository: kaidotio/github-actions-runner-controller
  tokenSecretKeyRef:
    name: credentials
    key: GITHUB_TOKEN

# This shows the image is pulling from the local docker registry
$ kubectl get pod -l app=example -o jsonpath='{$.items[*].metadata.name}: {$.items[*].spec.containers[0].image}'
example-6dd7c8974c-4sgjv: 127.0.0.1:31994/f601e6d⏎

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
  image: ubuntu:22.04
  repository: kaidotio/github-actions-runner-controller
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

## Development

```sh
$ make dev
```
