#!/usr/bin/env bash

set -e

USERNAME=${USERNAME:-devcontainer}

usermod -l ${USERNAME} vscode
usermod -d /home/${USERNAME} -m ${USERNAME}
usermod -c ${USERNAME} ${USERNAME}
groupmod -n ${USERNAME} vscode
mv /etc/sudoers.d/vscode /etc/sudoers.d/${USERNAME}
sed -i "s/vscode/${USERNAME}/g" /etc/sudoers.d/${USERNAME}
