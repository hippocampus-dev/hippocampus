#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

socat TCP-LISTEN:10080,fork,reuseaddr TCP:$(minikube ip):$(kubectl -n istio-gateways get svc istio-ingressgateway -o 'jsonpath={$.spec.ports[?(@.name=="http2")].nodePort}')
