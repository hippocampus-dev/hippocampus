#!/usr/bin/env bash

set -e

make build/${1}/x86_64-unknown-linux-gnu

cp target/x86_64-unknown-linux-gnu/release/${1} dist/${1}_linux_amd64_v1/${1}
