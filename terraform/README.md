# terraform

<!-- TOC -->
* [terraform](#terraform)
  * [Requirements](#requirements)
  * [Development](#development)
<!-- TOC -->

terraform is an Infrastructure as Code configuration for managing Google Cloud, Cloudflare, and GitHub resources.

## Requirements

- `gcloud` authenticated for the Google Cloud project, since `make apply` waits for the minikube GCE worker image builders through `gcloud compute instances` and sends them the registry credentials through `gcloud compute ssh --tunnel-through-iap`
- A running minikube with the `cilium` and `nvidia-device-plugin` DaemonSets in `kube-system`, since `make plan` and `make apply` read the Kubernetes, containerd and runc versions and the images the minikube GCE worker images bake from it
- `~/.docker/config.json` holding an inline `auths."ghcr.io".auth` entry, since `make apply` sends it to the minikube GCE worker image builders to pull the ghcr.io images they bake

## Development

```sh
$ make plan
```
