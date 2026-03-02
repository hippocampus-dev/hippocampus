#!/usr/bin/env bash

set -e

find /var/log/pods -type f -name "*.log.*" -delete
