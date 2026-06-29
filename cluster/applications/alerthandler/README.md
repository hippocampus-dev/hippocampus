# alerthandler

<!-- TOC -->
* [alerthandler](#alerthandler)
  * [Development](#development)
<!-- TOC -->

alerthandler is a Knative webhook service that processes Prometheus Alertmanager alerts, dispatching them to handlers for automatic pod remediation and GitHub issue creation.

## Development

```sh
$ make dev
```
