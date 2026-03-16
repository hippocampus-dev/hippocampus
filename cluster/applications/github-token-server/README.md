# github-token-server

<!-- TOC -->
* [github-token-server](#github-token-server)
  * [Development](#development)
<!-- TOC -->

github-token-server is a simple server that can be used to generate GitHub tokens for use in GitHub Actions workflows. It is designed to be run in a Kubernetes cluster and is configured to use the Kubernetes service account token to authenticate with the GitHub API.

## Development

```sh
$ make dev
```
