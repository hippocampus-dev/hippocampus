# github-actions-runner-controller

<!-- TOC -->
* [github-actions-runner-controller](#github-actions-runner-controller)
  * [Usage](#usage)
    * [Organization Runner](#organization-runner)
    * [Scale Set](#scale-set)
    * [Privileged Runner](#privileged-runner)
    * [Rootless Runner](#rootless-runner)
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
A `Runner` keeps one long-lived self-hosted runner, and a `ScaleSet` creates one ephemeral runner `Job` per assigned job.
The runner is based on the image `spec.image` names, which github-actions-runner-controller rebuilds as an image for the runner using [GoogleContainerTools/kaniko](https://github.com/GoogleContainerTools/kaniko) and distributes via local docker registry.

You can pass additional information to runner pod via `builderContainerSpec`, `runnerContainerSpec`, and `template`.

```yaml
spec:
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

To register runners at the organization level, omit the `repo` field, as `examples/runner-organization.yaml` does.

The scope is automatically inferred from the `repo` field:
- If `repo` is set: repository-level runner
- If `repo` is omitted: organization-level runner

### Scale Set

A `ScaleSet` takes the same fields as a `Runner` and adds `minRunners`, `maxRunners` and `runnerGroup`, as `examples/scale_set.yaml` does.
The controller keeps `min(maxRunners, minRunners + assigned jobs)` runners.
Its build and runner `Job`s are separate pods, so `template.spec.volumes` reaches the runner pod only where `runnerContainerSpec.volumeMounts` names it.

### Privileged Runner

`runnerContainerSpec.isolation: privileged` opens the container capability bounding set and voids the seccomp profile, which lets a workflow start a container runtime daemon inside it.
The runner process itself still holds none of those capabilities, so it reaches them through the `sudo` the rendered Dockerfile installs.
Nothing here ships that runtime or starts it: the image `spec.image` names carries it, and `runnerContainerSpec.env` is where a `ScaleSet` points `ACTIONS_RUNNER_HOOK_JOB_STARTED` at the script in that image that starts it (`cluster/applications/privileged-runner/README.md`).
Leave `template.spec.hostUsers` unset on such a runner, since the capabilities a pod's own user namespace carries hold against that namespace rather than against the node, and `cluster/applications/privileged-runner/start-dockerd.sh` reaches the node's loop devices through `mknod` and `losetup` whatever the bounding set holds.

```yaml
spec:
  image: <an image carrying the container runtime>
  runnerContainerSpec:
    isolation: privileged
  template:
    spec:
      runtimeClassName: kata-qemu
```

Whether such a runner is admitted at all is the namespace Pod Security Standard to settle, since `baseline` and `restricted` both reject the pod, and what those capabilities reach is the runtime class to settle, since a virtual machine one keeps them off the node kernel.

Two constraints follow that runtime class rather than this controller.
A virtual machine one serves the container filesystem over virtio-fs, which [cannot be the upper layer of overlayfs](https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-run-docker-with-kata.md), so a daemon keeping its images there falls back to vfs and copies every layer whole; mount an ext4 image on its storage directory instead.
`docker/setup-qemu-action` does not carry over either, since the `binfmt_misc` registration it installs is gone once the container installing it exits, and a `--platform` naming another architecture then fails with `exec format error`.

### Rootless Runner

`runnerContainerSpec.isolation: userns` serves a `docker build` through the rootless podman the rendered Dockerfile installs, without a daemon and without a runtime class of its own.
It adds `SYS_ADMIN`, `DAC_OVERRIDE` and `SYS_CHROOT` to the bounding set, unmasks `/proc`, and pins `template.spec.hostUsers` to false so those capabilities hold against the pod's own user namespace rather than against the node.

```yaml
spec:
  image: <any image>
  runnerContainerSpec:
    isolation: userns
```

The cluster settles whether the value does anything at all.
`UserNamespacesSupport` and `ProcMountType` both default on from Kubernetes 1.33, and below that the API server drops `hostUsers` and `procMount` rather than rejecting them, which would leave `SYS_ADMIN` held against the node; `pods.validating.kaidotio.github.io` in `cluster/manifests/validating-admission/base/validating_admission_policy.yaml` denies such a pod.
The runtime class settles it too, since a handler reports whether it supports user namespaces at all in the node's `status.runtimeHandlers` and `kata-qemu` reports that it does not.

Podman's storage falls to vfs, since fuse-overlayfs needs a `/dev/fuse` no pod carries, so size `runnerContainerSpec.resources.requests.ephemeral-storage` for layers copied whole rather than shared.

A `ScaleSet` builds its own image in a pod of its own, which keeps the node's user namespace, while a `Runner` builds it from an init container of the same pod and therefore inside the pod's user namespace, where Kaniko reaches neither a device node nor an owner outside the range the kubelet mapped.
Take `spec.image` on a `Runner` from a base image whose layers stay inside it.

### GitHub Apps

You can use GitHub Apps to authenticate the runner.

```sh
$ kubectl create secret generic credentials --from-literal=github_app_id="<YOUR GITHUB APP ID>" --from-literal=github_app_installation_id="<YOUR GITHUB APP INSTALLATION ID>" --from-file=github_app_private_key="<PATH TO YOUR GITHUB APP PRIVATE KEY>"
```

```yaml
spec:
  appSecretRef:
    name: credentials
```

#### Required Permissions

- Actions (read)
- Administration (read / write)
- Metadata (read)

## Development

```sh
$ make dev
```
