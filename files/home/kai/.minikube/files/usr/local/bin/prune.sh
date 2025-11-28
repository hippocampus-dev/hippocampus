#!/usr/bin/env bash

set -e

crictl ps -a | sed 1d | grep -v Running | awk '{print $1}' | xargs crictl rm
crictl rmi --prune
