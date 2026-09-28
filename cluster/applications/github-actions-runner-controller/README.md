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
A change to that image or to `runnerContainerSpec.isolation` reaches the runners only once the new image has been built, so leave a `ScaleSet` room for the build pod `builderContainerSpec` sizes, and a `Runner` room for one more runner pod, which carries that build.

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

The scope follows the `repo` field: set it for a repository-level runner, omit it for an organization-level one, as `examples/runner-organization.yaml` does.

`runnerGroup` names the GitHub runner group such a runner joins, and is rejected together with `repo`, since a runner group exists at the organization level only.
Name a group whose repository access policy lists the repositories the runner is meant to serve, since leaving it unset takes the group GitHub reports as the default, which every repository the organization holds can reach.

### Scale Set

A `ScaleSet` takes the same fields as a `Runner` and adds `minRunners` and `maxRunners`, as `examples/scale_set.yaml` does.
The controller keeps `min(maxRunners, minRunners + assigned jobs)` runners.
Its build and runner `Job`s are separate pods, so `template.spec.volumes` reaches the runner pod only where `runnerContainerSpec.volumeMounts` names it.
Runners keep coming from the last image that built while a new one is building, and a `ScaleSet` that has never built one takes no job until its first build succeeds.

A runner the cluster stops in the middle of a job, through an eviction, a preemption or a drain, leaves that workflow job in progress on GitHub with nothing to re-queue it.
The controller cancels the run and re-runs the job once for each such runner, and leaves an out-of-memory kill and a runner that failed on its own alone.

### Privileged Runner

`runnerContainerSpec.isolation: privileged` opens the container capability bounding set and voids the seccomp profile, which lets a workflow start a container runtime daemon inside it.
The runner process holds none of those capabilities itself and reaches them through the `sudo` the rendered Dockerfile installs.
Nothing here ships that runtime or starts it: the image `spec.image` names carries it, and `runnerContainerSpec.env` is where a `ScaleSet` points `ACTIONS_RUNNER_HOOK_JOB_STARTED` at the script in that image that starts it.

```yaml
spec:
  image: <an image carrying the container runtime>
  runnerContainerSpec:
    isolation: privileged
  template:
    spec:
      runtimeClassName: kata-qemu
```

Set the namespace Pod Security Standard to `privileged`, since `baseline` and `restricted` both reject the pod, and name a virtual machine runtime class to keep those capabilities off the node kernel.

Two constraints follow that runtime class rather than this controller.
Mount an ext4 image on the daemon's storage directory, since a virtual machine runtime class serves the container filesystem over virtio-fs, which [cannot be the upper layer of overlayfs](https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-run-docker-with-kata.md), and a daemon keeping its images there falls back to vfs and copies every layer whole.
`docker/setup-qemu-action` does not carry over, so a `--platform` naming another architecture fails with `exec format error`.

### Rootless Runner

`runnerContainerSpec.isolation: userns` serves a `docker build` through the rootless podman the rendered Dockerfile installs, without a daemon and without a runtime class of its own.
It needs `SYS_ADMIN`, `DAC_OVERRIDE` and `SYS_CHROOT`, an unmasked `/proc` and an unconfined seccomp profile, held against the pod's own user namespace rather than against the node.

```yaml
spec:
  image: <any image>
  runnerContainerSpec:
    isolation: userns
```

Three things outside this controller settle whether the value does anything at all.

| Setting | Requirement |
|---------|-------------|
| Kubernetes version | 1.33 or above, where `UserNamespacesSupport` and `ProcMountType` default on; below it the API server drops `hostUsers` and `procMount` and the pod is denied |
| Runtime class | One whose handler reports user namespace support in the node's `status.runtimeHandlers`, which `kata-qemu` does not |
| Pod Security Standard | `privileged`, since `baseline` and `restricted` both reject what this value adds |

Size `runnerContainerSpec.resources.requests.ephemeral-storage` for one copy of each layer podman stores.
Leave a job publishing a container port to the `privileged` isolation, since every container podman starts here joins the pod's own network namespace.
Take `spec.image` on a `Runner` from a base image whose layers carry no device node and no owner outside the range the kubelet maps.

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

- Actions (read / write)
- Administration (read / write)
- Metadata (read)

Write on Actions is what the cancel and the rerun are refused without, and nothing else here needs it.
Both are issued by the token `spec.tokenSecretKeyRef` names where a `ScaleSet` carries one and by the controller's own `--github-app-*` credentials where it does not, never by the App `spec.appSecretRef` names.

## Development

```sh
$ make dev
```
