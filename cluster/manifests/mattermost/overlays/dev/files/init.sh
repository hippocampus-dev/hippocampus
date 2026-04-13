#!/usr/bin/env bash

# Post-deploy initialization script for Mattermost.
# Run manually after first deployment:
#   kubectl exec -n mattermost mattermost-0 -c wait-for-database -- sh /mnt/init.sh
#
# Or use: kubectl run mattermost-init --rm -it --restart=Never \
#   --image=alpine:3.21 -n mattermost -- sh -c '
#     apk add --no-cache curl
#     until curl -sf http://mattermost:8065/api/v4/system/ping; do sleep 1; done
#     # Create team
#     curl -sf -X POST http://mattermost:8065/api/v4/teams \
#       -H "Content-Type: application/json" \
#       -d "{\"name\":\"mattermost\",\"display_name\":\"mattermost\",\"type\":\"O\"}"
#   '

set -e

MATTERMOST_URL="http://mattermost.mattermost.svc.cluster.local:8065"
FILE=/mattermost/data/webhookId

until nc -vz mattermost-vitess.mattermost.svc.cluster.local 3306 > /dev/null 2>&1; do sleep 1; done
