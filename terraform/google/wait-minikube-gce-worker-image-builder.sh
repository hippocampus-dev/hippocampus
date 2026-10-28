#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

TIMEOUT_SECONDS=1800

deadline=$(($(date +%s) + TIMEOUT_SECONDS))
# terraform/google/minikube-gce-worker-image.sh waits for this before pulling the private images
until [ "$(date +%s)" -ge "$deadline" ] || [ "$(gcloud compute instances describe "$INSTANCE" --zone "$ZONE" --project "$PROJECT" --format "value(status)")" = TERMINATED ] || gcloud compute ssh "$INSTANCE" --zone "$ZONE" --project "$PROJECT" --tunnel-through-iap --quiet --command "sudo install -m 600 /dev/stdin /run/registry-credentials.json.tmp && sudo mv /run/registry-credentials.json.tmp /run/registry-credentials.json" < ~/.docker/config.json; do
  sleep 10
done
until [ "$(date +%s)" -ge "$deadline" ] || [ "$(gcloud compute instances describe "$INSTANCE" --zone "$ZONE" --project "$PROJECT" --format "value(status)")" = TERMINATED ]; do
  sleep 10
done
status=$(gcloud compute instances describe "$INSTANCE" --zone "$ZONE" --project "$PROJECT" --format "value(status)")
if [ "$status" != TERMINATED ]; then
  echo "${INSTANCE} did not reach TERMINATED within ${TIMEOUT_SECONDS}s" >&2
  gcloud compute instances get-serial-port-output "$INSTANCE" --zone "$ZONE" --project "$PROJECT" | tail -n 100 >&2 || true
  gcloud compute instances delete "$INSTANCE" --zone "$ZONE" --project "$PROJECT" --quiet
  exit 1
fi
