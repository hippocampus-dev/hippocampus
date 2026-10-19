#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   e2e.sh [NAME...]

Serves each pipeline and upscaler in config.yaml, or only those named, and
sends photos of properties listed on LIFULL HOME'S to /v1/masks and /v1/remove
through each pipeline, to /v1/replace and /v1/insert, and photos of people on
Wikimedia Commons to /v1/background, through those that support them, and to
/v1/upscale through each upscaler, writing what each returns to results/NAME/.
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

cd "$(dirname "${BASH_SOURCE[0]}")"

readonly IMAGES_DIR=images
readonly RESULTS_DIR=results
export PORT=58080
readonly IMAGES=(
  5509/sale/8/11/3/hmli
  109083/sale/5092/11/1/0a6u
  154820/sale/3224/11/4/9e4g
  139283/sale/1487/11/2/uocq
  109083/sale/5125/11/4/kv9e
  109083/sale/5055/11/1/70sr
)
# Photos of people on Wikimedia Commons under CC BY-SA
readonly PEOPLE=(
  portrait=f/ff/2018-01-27_-_Ester_Bejarano_-_3080.jpg
  wedding=b/b2/2016_Prowincja_Krabi%2C_Ko_Phi_Phi_Don%2C_Pla%C5%BCa_Loh_Moo_Dee%2C_Tajska_para_%C5%9Blubna_%2804%29.jpg
  vendor=3/3f/A_bad_sales_day.jpg
  snowboard=d/d4/2018_Snowboardzistka.jpg
)
readonly FURNITURE="sofa. chair. stool. table. rug. cushion. plant. vase. floor lamp. tv stand. shelf. bowl."
TORCH_BACKEND="${TORCH_BACKEND:-cu130}"

mkdir -p "$IMAGES_DIR"
for image in "${IMAGES[@]}"; do
  name="$(basename "$image")"
  if [ -s "${IMAGES_DIR}/${name}.jpg" ]; then
    continue
  fi
  # Downloaded aside and moved on success, so an interrupted download is not taken for a complete one on the next run
  curl -fsSL --retry 5 --retry-all-errors -G -o "${IMAGES_DIR}/${name}.jpg.partial" \
    --data-urlencode "file=http://img.homes.jp/${image}.jpg" -d width=1600 -d height=1600 \
    https://image.homes.jp/smallimg/image.php
  mv "${IMAGES_DIR}/${name}.jpg.partial" "${IMAGES_DIR}/${name}.jpg"
done
for person in "${PEOPLE[@]}"; do
  name="${person%%=*}"
  path="${person#*=}"
  if [ -s "${IMAGES_DIR}/${name}.jpg" ]; then
    continue
  fi
  curl -fsSL --retry 5 --retry-all-errors -o "${IMAGES_DIR}/${name}.jpg.partial" \
    "https://upload.wikimedia.org/wikipedia/commons/thumb/${path}/1920px-$(basename "$path")"
  mv "${IMAGES_DIR}/${name}.jpg.partial" "${IMAGES_DIR}/${name}.jpg"
done

entries=$(uv run --extra "$TORCH_BACKEND" -- python -c 'import yaml; c = yaml.safe_load(open("config.yaml")); [print("PIPELINE", p["name"], p["inpainter"]["type"]) for p in c.get("pipelines", [])]; [print("UPSCALER", u["name"], u["type"]) for u in c.get("upscalers", [])]')
for entry in "${args[@]}"; do
  if [[ ! $'\n'"$entries" =~ $'\n'[A-Z]+" ${entry} " ]]; then
    echo "Unknown pipeline or upscaler $entry" 1>&2
    exit 1
  fi
done

function request() {
  local name="$1" endpoint="$2"
  shift 2
  echo "${entry}/${name}"
  curl -fsS --retry 5 --retry-all-errors -o "${RESULTS_DIR}/${entry}/${name}.png" "$@" "http://127.0.0.1:${PORT}/v1/${endpoint}"
}

while IFS=" " read -r variable entry type; do
  if [ "${#args[@]}" -gt 0 ] && [[ ! " ${args[*]} " =~ " ${entry} " ]]; then
    continue
  fi

  mkdir -p "${RESULTS_DIR}/${entry}"
  env -u PIPELINE -u UPSCALER "${variable}=${entry}" PYTORCH_ALLOC_CONF=pinned_max_round_threshold_mb:0 uv run --extra "$TORCH_BACKEND" -- python main.py &
  pid=$!
  trap 'kill "$pid" 2>/dev/null || true' EXIT
  while ! curl -fsSo /dev/null "http://127.0.0.1:${PORT}/healthz" 2>/dev/null; do
    kill -0 "$pid"
    sleep 1
  done

  if [ "$variable" = UPSCALER ]; then
    request 0a6u-upscale upscale -F "image=@${IMAGES_DIR}/0a6u.jpg" -F scale=2
  else
    for image in "${IMAGES[@]}"; do
      name="$(basename "$image")"
      request "${name}-masks" masks -F "image=@${IMAGES_DIR}/${name}.jpg" -F "prompt=${FURNITURE}"
      request "${name}-remove" remove -F "image=@${IMAGES_DIR}/${name}.jpg" -F "prompt=${FURNITURE}"
    done
    request 9e4g-remove-text remove -F "image=@${IMAGES_DIR}/9e4g.jpg" -F text=true
    request kv9e-remove-keep remove -F "image=@${IMAGES_DIR}/kv9e.jpg" -F "prompt=${FURNITURE}" -F "keep=kitchen counter."
    if [ "$type" = gpt-image ] || [ "$type" = qwen-image-edit ]; then
      request uocq-replace replace -F "image=@${IMAGES_DIR}/uocq.jpg" -F "prompt=rug." -F "content=a round beige wool rug" -F "keep=side table."
      request hmli-insert insert -F "image=@${IMAGES_DIR}/hmli.jpg" -F "prompt=tv stand." -F relation=on -F "content=a flat-screen TV" -F "keep=picture frame. potted plant."
      request portrait-background background -F "image=@${IMAGES_DIR}/portrait.jpg" -F "prompt=person." -F "content=a bright modern office with large windows"
      request portrait-sunset-background background -F "image=@${IMAGES_DIR}/portrait.jpg" -F "prompt=person." -F "content=a beach at sunset with warm orange light"
      request wedding-background background -F "image=@${IMAGES_DIR}/wedding.jpg" -F "prompt=person." -F "content=a garden with blooming roses and a stone fountain"
      request vendor-background background -F "image=@${IMAGES_DIR}/vendor.jpg" -F "prompt=person. basket of snacks." -F "content=a quiet street in a European old town"
      request snowboard-background background -F "image=@${IMAGES_DIR}/snowboard.jpg" -F "prompt=person. snowboard." -F "content=a sunny beach with palm trees"
    fi
  fi

  kill "$pid"
  wait "$pid" || true
done <<< "$entries"

echo "e2e: ok"
