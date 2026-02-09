#!/usr/bin/env -S bash -l

set -eo pipefail

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
  policyName: block-all-pods
  validationActions: [Deny]
EOF

kubectl delete pod --all --all-namespaces --grace-period=0

minikube stop
