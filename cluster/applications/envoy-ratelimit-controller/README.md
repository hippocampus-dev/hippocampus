# envoy-ratelimit-controller

<!-- TOC -->
* [envoy-ratelimit-controller](#envoy-ratelimit-controller)
  * [Development](#development)
<!-- TOC -->

envoy-ratelimit-controller is a Kubernetes controller that reconciles each RateLimit custom resource into an EnvoyFilter and an envoy-ratelimit config served over xDS, reporting whether envoy-ratelimit accepted it.

## Development

```sh
$ make dev
```
