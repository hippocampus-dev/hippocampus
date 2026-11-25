#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

go test -tags e2e -count=1 -run TestE2E .
