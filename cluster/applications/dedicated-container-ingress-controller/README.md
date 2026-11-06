# dedicated-container-ingress-controller

<!-- TOC -->
* [dedicated-container-ingress-controller](#dedicated-container-ingress-controller)
  * [Development](#development)
<!-- TOC -->

dedicated-container-ingress-controller is an ingress controller whose reverse proxy runs a dedicated pod per client session from the pod template declared in a DedicatedContainerIngress resource.

## Development

```sh
$ make dev
```
