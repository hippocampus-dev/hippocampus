#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR
trap 'minikube stop; sync' EXIT

# cluster-autoscaler, the only thing shrinking minikube-gce-worker.sh's instance groups, stops with the cluster
project=$(gcloud config get-value project)
if [ -n "$project" ]; then
  gcloud compute instance-groups managed resize minikube-gce-worker --zone asia-northeast1-a --size 0
  gcloud compute instance-groups managed resize minikube-gce-worker-gpu --zone asia-northeast1-a --size 0
fi

/usr/local/bin/hyperv-host.sh stop

referenced=$(
  {
    kubectl get pods --all-namespaces -o jsonpath='{range .items[*]}{..imageID} {..image}{"\n"}{end}' &&
      kubectl get deployments,statefulsets,daemonsets,cronjobs,jobs --all-namespaces -o jsonpath='{range .items[*]}{..image}{"\n"}{end}' &&
      # minikube start re-extracts the preload tarball over /var/lib/containerd once any kubeadm image is missing (https://github.com/kubernetes/minikube/blob/v1.38.1/pkg/minikube/cruntime/containerd.go)
      minikube ssh -- 'for directory in /var/lib/minikube/binaries/*/; do sudo "${directory}kubeadm" config images list --kubernetes-version "$(basename "$directory")"; done' | tr -d '\r'
  } | tr ' ' '\n' | awk 'NF' | sort -u
)
if [ -n "$referenced" ]; then
  images=$(minikube ssh -- sudo crictl --timeout=10s images -o json)
  unreferenced=$(jq -r --arg referenced "$referenced" '
    ($referenced | split("\n")) as $references
    | ($references | map(select(test("@sha256:")) | sub(".*@"; ""))) as $digests
    | ($references | map(select(test("^sha256:")))) as $ids
    | ($references | map(select(test("@|^sha256:") | not))) as $tags
    | .images[]
    | select(.pinned | not)
    | select(.id | IN($ids[]) | not)
    | select(any(.repoDigests[]?; sub(".*@"; "") | IN($digests[])) | not)
    | select(any(.repoTags[]?; IN($tags[])) | not)
    | .id
  ' <<< "$images")
  if [ -n "$unreferenced" ]; then
    minikube ssh -- sudo crictl --timeout=5m rmi $unreferenced
  fi
else
  echo "no referenced image found" 1>&2
fi

minikube ssh -- sudo bash /usr/local/bin/prune.sh

cat <<EOS | kubectl apply -f -
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
EOS

# https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/2000-graceful-node-shutdown
#kubectl delete pod --all --all-namespaces --grace-period=0

minikube ssh -- "sudo systemctl stop kubelet && sudo systemctl stop containerd"
minikube ssh -- sync
minikube stop
