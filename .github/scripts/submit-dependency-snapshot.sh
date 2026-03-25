#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   submit-dependency-snapshot.sh <sbom.cyclonedx.json>

Options:
   -h, --help    Show this help
EOS
}

args=()
flags=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

sbom="${args[0]:-}"

if [ ! -f "$sbom" ]; then
  usage 1>&2
  exit 1
fi

image=$(jq -re '.metadata.component.name // empty' "$sbom")
scanned=$(jq -re '.metadata.timestamp' "$sbom")

snapshot_file=$(mktemp)

jq -n \
  --arg sha "$GITHUB_SHA" \
  --arg ref "$GITHUB_REF" \
  --arg correlator "$GITHUB_WORKFLOW $GITHUB_JOB" \
  --arg run_id "$GITHUB_RUN_ID" \
  --arg image "$image" \
  --arg scanned "$scanned" \
  --slurpfile sbom "$sbom" \
  '{
    version: 0,
    sha: $sha,
    ref: $ref,
    job: { correlator: $correlator, id: $run_id },
    detector: { name: "trivy", url: "https://github.com/aquasecurity/trivy", version: "0.34.2" },
    scanned: $scanned,
    manifests: {
      ($image): {
        name: $image,
        resolved: (
          [$sbom[0].components // [] | .[] | select(.purl != null) |
            { key: .name, value: { package_url: .purl, relationship: "direct" } }
          ] | from_entries
        )
      }
    }
  }' > "$snapshot_file"

curl -fsSL -X POST -H "Authorization: token $GH_TOKEN" -H "Accept: application/vnd.github+json" "https://api.github.com/repos/${GITHUB_REPOSITORY}/dependency-graph/snapshots" -d "@${snapshot_file}" -o /dev/null

component_count=$(jq '[.components // [] | .[] | select(.purl != null)] | length' "$sbom")
echo "Submitted dependency snapshot for $image (${component_count} components)"
