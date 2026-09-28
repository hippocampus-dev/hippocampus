#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# minikube-socat.service is ordered after minikube.service alone, so the gateway is still absent when this starts
until node_port=$(kubectl -n istio-gateways get svc istio-ingressgateway -o 'jsonpath={$.spec.ports[?(@.name=="http2")].nodePort}' 2> /dev/null) && [ -n "$node_port" ]; do
  sleep 5
done

node_ip=$(minikube ip)

socat TCP-LISTEN:10080,fork,reuseaddr "TCP:${node_ip}:${node_port}"
