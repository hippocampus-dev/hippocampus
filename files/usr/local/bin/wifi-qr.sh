#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly MODULE_SIZE=30

usage() {
  cat <<EOS
Usage:
   wifi-qr.sh <ssid> [output]

Arguments:
   ssid      WPA network name; the password is prompted for
   output    PNG file to write (default: wifi-qr.png)

Options:
   -h, --help   Show this help
EOS
}

args=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

if [ "${#args[@]}" -lt 1 ] || [ "${#args[@]}" -gt 2 ]; then
  usage 1>&2
  exit 1
fi
ssid="${args[0]}"
output="${args[1]:-wifi-qr.png}"

IFS= read -rsp "Password: " password
echo 1>&2

# https://github.com/zxing/zxing/wiki/Barcode-Contents
escape() {
  printf '%s' "$1" | sed 's/[\\;,:"]/\\&/g'
}

escaped_ssid=$(escape "$ssid")
escaped_password=$(escape "$password")
printf '%s' "WIFI:T:WPA;S:${escaped_ssid};P:${escaped_password};;" | qrencode -s "$MODULE_SIZE" -o "$output"
