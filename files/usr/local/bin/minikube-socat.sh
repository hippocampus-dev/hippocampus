#!/usr/bin/env bash

set -eo pipefail

socat TCP-LISTEN:10080,fork,reuseaddr TCP:$(minikube ip):$(kubectl -n istio-system get svc istio-ingressgateway -o 'jsonpath={$.spec.ports[?(@.name=="http2")].nodePort}')
