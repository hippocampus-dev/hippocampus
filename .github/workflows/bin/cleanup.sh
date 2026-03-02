#!/usr/bin/env bash

set -e

df -h

docker system prune -af --volumes
sudo rm -rf ${AGENT_TOOLSDIRECTORY}
sudo rm -rf /opt/az
sudo rm -rf /opt/microsoft
sudo rm -rf /opt/google
sudo rm -rf /opt/pipx
sudo rm -rf /opt/mssql-tools
sudo rm -rf /usr/lib/jvm
sudo rm -rf /usr/lib/google-cloud-sdk
sudo rm -rf /usr/lib/gcc
sudo rm -rf /usr/lib/llvm-*
sudo rm -rf /usr/lib/mono
sudo rm -rf /usr/lib/heroku
sudo rm -rf /usr/lib/firefox
sudo rm -rf /usr/lib/python3
sudo rm -rf /usr/share/dotnet
sudo rm -rf /usr/share/swift
sudo rm -rf /usr/share/miniconda
sudo rm -rf /usr/share/az_*
sudo rm -rf /usr/share/gradle-*
sudo rm -rf /usr/share/sbt
sudo rm -rf /usr/share/kotlinc
sudo rm -rf /usr/local/lib/android
sudo rm -rf /usr/local/lib/node_modules
sudo rm -rf /usr/local/share/powershell
sudo rm -rf /usr/local/share/chromium
sudo rm -rf /usr/local/share/vcpkg

df -h
du -sh /opt/* | sort -rh | head -n 5
du -sh /usr/lib/* | sort -rh | head -n 5
du -sh /usr/share/* | sort -rh | head -n 5
du -sh /usr/local/lib/* | sort -rh | head -n 5
du -sh /usr/local/share/* | sort -rh | head -n 5
