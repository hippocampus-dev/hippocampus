# privileged-runner

<!-- TOC -->
* [privileged-runner](#privileged-runner)
  * [Development](#development)
<!-- TOC -->

privileged-runner is a runner image carrying a container runtime, for a `ScaleSet` whose `runnerContainerSpec.isolation` is `privileged`.
`start-dockerd.sh` starts that runtime through the `sudo` the rendered Dockerfile installs, from the job started hook the `ScaleSet` names in `ACTIONS_RUNNER_HOOK_JOB_STARTED`, which actions/runner runs before it creates a job's `container:` and `services:`.

## Development

```sh
$ make all
```
