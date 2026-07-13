#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR
trap 'minikube stop; sync' EXIT

minikube ssh -- sudo bash /usr/local/bin/prune.sh

cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: temporary.validating.kaidotio.github.io
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:
          - ""
        apiVersions:
          - v1
        operations:
          - CREATE
        resources:
          - pods
  validations:
    - expression: "false"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: temporary.validating.kaidotio.github.io
spec:
  policyName: temporary.validating.kaidotio.github.io
  validationActions: [Deny]
EOF

# https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/2000-graceful-node-shutdown
#kubectl delete pod --all --all-namespaces --grace-period=0

minikube ssh -- "sudo systemctl stop kubelet && sudo systemctl stop containerd"
minikube ssh -- sync
minikube stop
