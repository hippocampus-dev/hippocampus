#!/usr/bin/env bash

set -e

REPOSITORY=hippocampus-dev/hippocampus

t=$(mktemp -d)

cd $t

if [ -z "${GITHUB_TOKEN}" ]; then
    echo "Please declare required environment variables: GITHUB_TOKEN" 1>&2
    exit 1
fi

token=$(curl -fsSL -X POST -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/runners/registration-token | jq -r .token)

if [ -z "${token}" ]; then
    echo "Error: Failed to get registration token"
    exit 1
fi

version=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/actions/runner/releases/latest | jq -r .tag_name | sed 's/v//')

curl -fsSL https://github.com/actions/runner/releases/download/v${version}/actions-runner-linux-x64-${version}.tar.gz -o actions-runner-linux-x64.tar.gz
tar xzf actions-runner-linux-x64.tar.gz
rm actions-runner-linux-x64.tar.gz

./config.sh --url https://github.com/${REPOSITORY} --token $token --name local-runner-$(date +%s.%N) --work $(mktemp -d) --labels self-hosted,Linux,X64,local --unattended

ENV_OVERRIDE_C=$(mktemp --suffix=.c)
ENV_OVERRIDE_SO=$(mktemp --suffix=.so)

cat <<EOF > $ENV_OVERRIDE_C
#define _GNU_SOURCE
#include <string.h>
#include <dlfcn.h>

char *getenv(const char *name) {
    char *(*original_getenv)(const char *) = (char *(*)(const char *))dlsym(RTLD_NEXT, "getenv");

    if (strcmp(name, "ANTHROPIC_API_KEY") == 0) return "";

    return original_getenv(name);
}
EOF

gcc -shared -fPIC -o $ENV_OVERRIDE_SO $ENV_OVERRIDE_C -ldl

cleanup() {
    if [ -f .runner ]; then
        token=$(curl -fsSL -X POST -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/runners/registration-token | jq -r .token)

        if [ -z "${token}" ]; then
            echo "Error: Failed to get registration token"
            exit 1
        fi

        ./config.sh remove --token $token
    fi
    rm -f $ENV_OVERRIDE_C
    rm -f $ENV_OVERRIDE_SO
    exit 0
}

trap cleanup EXIT

LD_PRELOAD=$ENV_OVERRIDE_SO ./run.sh
