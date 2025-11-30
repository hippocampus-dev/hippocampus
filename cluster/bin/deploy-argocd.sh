#!/usr/bin/env bash

set -e

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

kubectl apply -k ${ENTRYPOINT}/../manifests/argocd/overlays/dev
until kubectl -n argocd wait --for=condition=Ready pod --all --timeout=10m; do sleep 1; done
kubectl get secret argocd-credentials -n argocd 2> /dev/null || kubectl create secret generic argocd-credentials --from-file=sshPrivateKey=/home/${USER}/.ssh/github -n argocd

cat <<EOS | kubectl apply -f -
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
  namespace: argocd
spec:
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
  destinations:
    - namespace: '*'
      server: '*'
  sourceRepos:
    - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: argocd-applications
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: git@github.com:hippocampus-dev/hippocampus
    targetRevision: main
    path: cluster/manifests/argocd-applications/overlays/dev
    kustomize:
      version: custom
      commonAnnotationsEnvsubst: true
      commonAnnotations:
        repository: ${ARGOCD_APP_SOURCE_REPO_URL}
        # `revision` annotation is injected by https://github.com/hippocampus-dev/hippocampus/blob/main/cluster/manifests/argocd/overlays/dev/patches/deployment.yaml to support monorepo
        #revision: ${ARGOCD_APP_REVISION}
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: false
    syncOptions:
      - Validate=true
      - CreateNamespace=false
EOS
