# tls-intercept-proxy

<!-- TOC -->
* [tls-intercept-proxy](#tls-intercept-proxy)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

tls-intercept-proxy is a forward proxy that terminates TLS with certificates issued on the fly so that a plain HTTP backend can handle requests a client sends over HTTPS before they reach the original host.

## Usage

The forward proxy listener answers `CONNECT`, issues a certificate for the requested host from the configured certificate authority, and forwards the decrypted request to the remote address with the original scheme and host in `X-Original-Scheme` and `X-Original-Host`; a request that reaches the same listener without `CONNECT` takes the same route with `X-Original-Scheme: http`.
The reverse proxy listener is where those requests come back after the remote address is done with them: it reads those two headers and reaches the original host under the scheme they name.

```
client
  │ CONNECT example.com:443, then TLS answered with a certificate issued for example.com
  ▼
forward proxy :3128
  │ http://remote-address/, Host: remote-host
  │ X-Original-Scheme: https, X-Original-Host: example.com
  ▼
remote address (remote-address)
  │ forwarded on with both headers still on the request
  ▼
reverse proxy :8080
  │ https://example.com/, Host: example.com, both headers removed
  ▼
example.com
```

```sh
$ tls-intercept-proxy \
    --local-forward-proxy-address=0.0.0.0:3128 \
    --local-reverse-proxy-address=0.0.0.0:8080 \
    --remote-address=varnish:6081 \
    --remote-host=varnish \
    --certificate-authority-file=/var/certs/tls.crt \
    --certificate-authority-private-key-file=/var/certs/tls.key
```

Clients must trust the certificate authority.
Chromium accepts it through `--ignore-certificate-errors-spki-list=<base64 encoded SHA-256 of the certificate authority SubjectPublicKeyInfo>`.

## Development

```sh
$ export CERTIFICATE_AUTHORITY_FILE=<path>
$ export CERTIFICATE_AUTHORITY_PRIVATE_KEY_FILE=<path>
$ make dev
```
