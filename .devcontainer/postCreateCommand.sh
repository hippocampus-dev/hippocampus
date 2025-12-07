#!/usr/bin/bash

set -e

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

export DEBIAN_FRONTEND=noninteractive

bash ${ENTRYPOINT}/../setup/ubuntu/install-packages.sh

chown -R ${LOCAL_USER}:${LOCAL_USER} /home/${LOCAL_USER}

sudo mkdir /var/certs
sudo chown -R ${LOCAL_USER}:${LOCAL_USER} /var/certs

sudo -u ${LOCAL_USER} bash -l ${ENTRYPOINT}/../setup/asdf.sh
sudo -u ${LOCAL_USER} bash -l ${ENTRYPOINT}/../setup/user.sh
