#!/usr/bin/env bash

password=$(openssl rand -hex 16)
export CHISEL_AUTH="nonroot:${password}"

cat > /home/nonroot/.bashrc <<EOF
banner_marker="\${XDG_RUNTIME_DIR:-/tmp}/remotty-banner-shown"
if [ ! -f "\$banner_marker" ]; then
  echo "============================================"
  echo " Remotty Workspace"
  echo "============================================"
  echo ""
  echo " Chisel Auth: nonroot:${password}"
  echo ""
  echo " Usage:"
  echo "   armyknife remotty --auth nonroot:${password} https://remotty.kaidotio.dev/tunnel R:<remote-port>:127.0.0.1:<local-port>"
  echo "   armyknife remotty --auth nonroot:${password} https://remotty.kaidotio.dev/tunnel 127.0.0.1:<local-port>:<remote-host>:<remote-port>"
  echo "   armyknife remotty --auth nonroot:${password} --env LANG,LC_* https://remotty.kaidotio.dev/tunnel"
  echo ""
  echo "============================================"
  : > "\$banner_marker"
fi

remotty-env() {
  local out
  if out=\$(curl -fsS -u "\$CHISEL_AUTH" http://127.0.0.1:65532/v1/env 2>/dev/null); then
    eval "\$out"
  fi
}

remotty-env
EOF

exec supervisord -c /etc/supervisord.conf
