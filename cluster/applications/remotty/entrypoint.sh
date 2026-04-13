#!/usr/bin/env bash

password=$(openssl rand -hex 16)
export CHISEL_AUTH="nonroot:${password}"

cat > /home/nonroot/.bashrc <<EOF
echo "============================================"
echo " Remotty Workspace"
echo "============================================"
echo ""
echo " Chisel Auth: nonroot:${password}"
echo ""
echo " Usage:"
echo "   armyknife remotty --auth nonroot:${password} https://remotty.kaidotio.dev/tunnel R:<remote-port>:localhost:<local-port>"
echo ""
echo "============================================"
EOF

exec supervisord -c /etc/supervisord.conf
