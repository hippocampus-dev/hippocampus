#!/usr/bin/env -S bash -l

set -e

socat TCP-LISTEN:10080,fork,reuseaddr TCP:$(minikube ip):$(kubectl -n istio-gateways get svc istio-ingressgateway -o 'jsonpath={$.spec.ports[?(@.name=="http2")].nodePort}')
