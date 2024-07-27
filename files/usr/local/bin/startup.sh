#!/usr/bin/env bash

set -eo pipefail

f=$(mktemp)

pacman-key --refresh --keyserver hkp://keyserver.ubuntu.com >> $f

cat $f > /var/log/startup
