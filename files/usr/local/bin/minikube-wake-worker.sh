#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   minikube-wake-worker.sh <node>

Arguments:
   node   Worker node whose wake-on-lan.kaidotio.github.io/mac annotation names the machine to wake

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

if [ "${#args[@]}" -ne 1 ]; then
  usage >&2
  exit 1
fi

mac=$(minikube kubectl -- get node "${args[0]}" -o jsonpath='{.metadata.annotations.wake-on-lan\.kaidotio\.github\.io/mac}')
# A node writes its own annotations
# printf turns anything but hex digit pairs into other bytes without failing
if [[ ! "$mac" =~ ^[0-9a-f]{2}(:[0-9a-f]{2}){5}$ ]]; then
  echo "Node ${args[0]} has no valid wake-on-lan.kaidotio.github.io/mac annotation: ${mac}" >&2
  exit 1
fi

# https://en.wikipedia.org/wiki/Wake-on-LAN#Magic_packet
format='\xff\xff\xff\xff\xff\xff'
for i in $(seq 16); do
  format+="\\x${mac//:/\\x}"
done
# socat sends each read as its own datagram: https://man.archlinux.org/man/socat.1.en
# bash line-buffers stdout, so printf writes again after every 0x0a byte
printf "$format" | dd bs=4096 count=1 iflag=fullblock status=none | socat - UDP-DATAGRAM:255.255.255.255:9,broadcast
