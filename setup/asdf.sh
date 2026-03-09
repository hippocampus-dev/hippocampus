#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

mkdir -p ~/.asdf
(
  cd ~/.asdf
  git init
  git remote add origin https://github.com/asdf-vm/asdf.git
  git pull origin v0.14.1 --depth 1
)

cat <<EOS | sudo tee /etc/profile.d/asdf.sh > /dev/null
. /home/${USER}/.asdf/asdf.sh
EOS
