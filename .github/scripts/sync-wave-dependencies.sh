#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPOSITORY=$(cd "${ENTRYPOINT}/../.." && pwd)
APPLICATIONS="${REPOSITORY}/cluster/manifests/argocd-applications/base"

# cluster/bin/deploy-argocd.sh applies the argocd overlay before the app-of-apps Application exists
BOOTSTRAPPED_NAMESPACES="argocd"

WORKSPACE=$(mktemp -d)
trap 'rm -rf "${WORKSPACE}"' EXIT

DECLARATION='
  /^  name: / { name = $2 }
  /argocd\.argoproj\.io\/sync-wave:/ { gsub(/"/, "", $2); wave = $2 }
  /^    path: / { path = $2 }
  END { if (name != "") print name "\t" (wave == "" ? 0 : wave) "\t" path }
'

# A CustomResourceDefinition is keyed by spec.names.kind with spec.group rather than by the group alone, because tetragon defines a CustomResourceDefinition in cilium.io while the Cilium resources every workload uses come from cluster/bin/deploy-cilium.sh
REFERENCE='
  function flush() {
    if (kind == "Namespace") owns["namespace/" name] = 1
    else if (kind == "CustomResourceDefinition") owns["resource/" definedKind "." definedGroup] = 1
    else {
      if (namespace != "") uses["namespace/" namespace] = 1
      if (group != "") uses["resource/" kind "." group] = 1
    }
    kind = ""; group = ""; name = ""; namespace = ""; definedKind = ""; definedGroup = ""
  }
  /^---$/ { flush(); next }
  /^[^[:space:]]/ { block = $1; section = "" }
  block == "kind:" { kind = $2 }
  block == "apiVersion:" { split($2, version, "/"); group = (version[2] == "" ? "" : version[1]) }
  block == "metadata:" && /^  name: / { name = $2 }
  block == "metadata:" && /^  namespace: / { namespace = $2 }
  block == "spec:" && /^  [^ ]/ { section = $1 }
  block == "spec:" && /^  group: / { definedGroup = $2 }
  block == "spec:" && section == "names:" && /^    kind: / { definedKind = $2 }
  END {
    flush()
    for (reference in owns) print "owns\t" reference
    for (reference in uses) if (!(reference in owns)) print "uses\t" reference
  }
'

while IFS= read -r declaration; do
  awk "${DECLARATION}" "${declaration}"
done < <(find "${APPLICATIONS}" -name "*.yaml" -not -name "kustomization.yaml" | LC_ALL=C sort) > "${WORKSPACE}/applications"

while IFS=$'\t' read -r name wave path; do
  kustomize build --enable-alpha-plugins "${REPOSITORY}/${path}" | awk "${REFERENCE}" | sed "s/^/${name}\t/"
done < "${WORKSPACE}/applications" > "${WORKSPACE}/references"

awk -F'\t' -v bootstrapped="${BOOTSTRAPPED_NAMESPACES}" '
  BEGIN {
    split(bootstrapped, entries, " ")
    for (entry in entries) excluded["namespace/" entries[entry]] = 1
  }
  NR == FNR { wave[$1] = $2; next }
  $2 == "owns" { owner[$3] = $1; next }
  { dependent[NR] = $1; reference[NR] = $3 }
  END {
    for (i in dependent) {
      if (reference[i] in excluded) continue
      dependency = owner[reference[i]]
      if (dependency == "" || dependency == dependent[i]) continue
      print dependent[i] "\t" wave[dependent[i]] "\t" dependency "\t" wave[dependency] "\t" reference[i]
    }
  }
' "${WORKSPACE}/applications" "${WORKSPACE}/references" | LC_ALL=C sort -u
