# gitlab-sso-bridge

<!-- TOC -->
* [gitlab-sso-bridge](#gitlab-sso-bridge)
  * [Development](#development)
<!-- TOC -->

gitlab-sso-bridge is an OAuth 2.0 authorization server that takes the caller's identity from a request header only a route restricted to an ext-authz host makes trustworthy, and serves the user API in the shape Mattermost's GitLab SSO expects.

## Development

```sh
$ export CLIENT_ID=<value>
$ export CLIENT_SECRET=<value>
$ export REDIRECT_URI=<value>
$ make dev
```
