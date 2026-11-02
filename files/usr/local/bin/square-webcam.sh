#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

SINK_DEVICE=/dev/video10
SOURCE_DEVICE=""
SOURCE_INPUT_FORMAT=mjpeg
SOURCE_VIDEO_SIZE=1920x1080
FRAMERATE=30
PLACEHOLDER_FRAMERATE=1
CONSUMER_SETTLE_SECONDS=2
RECONCILE_SECONDS=30

detect_source_device() {
  for device in /dev/video*; do
    if [ "$device" = "$SINK_DEVICE" ]; then
      continue
    fi
    # Device Caps is the node's own capability set - a UVC metadata node carries Video Capture in Driver Caps too
    capabilities=$(v4l2-ctl -d "$device" --info 2>/dev/null | awk '/Device Caps/,0') || true
    case "$capabilities" in
      *"Video Capture"*)
        echo "$device"
        return 0
        ;;
    esac
  done
  return 1
}

has_consumer() {
  openers=$(fuser "$SINK_DEVICE" 2>/dev/null) || true
  for pid in $openers; do
    if [ "$pid" != "$producer_pid" ]; then
      return 0
    fi
  done
  return 1
}

is_producing() {
  if [ -z "$producer_pid" ]; then
    return 1
  fi
  kill -0 "$producer_pid" 2>/dev/null
}

start_camera_producer() {
  source_device="$SOURCE_DEVICE"
  if [ -z "$source_device" ]; then
    source_device=$(detect_source_device)
  fi
  ffmpeg -nostdin -loglevel error \
    -f v4l2 -input_format "$SOURCE_INPUT_FORMAT" -video_size "$SOURCE_VIDEO_SIZE" -framerate "$FRAMERATE" -i "$source_device" \
    -vf crop=ih:ih \
    -f v4l2 -pix_fmt yuv420p "$SINK_DEVICE" &
  producer_pid=$!
  producer_kind=camera
}

start_placeholder_producer() {
  ffmpeg -nostdin -loglevel error \
    -re -f lavfi -i "color=c=black:s=${sink_side}x${sink_side}:r=${PLACEHOLDER_FRAMERATE}" \
    -f v4l2 -pix_fmt yuv420p "$SINK_DEVICE" &
  producer_pid=$!
  producer_kind=placeholder
}

stop_producer() {
  if is_producing; then
    kill "$producer_pid"
    wait "$producer_pid" || true
  fi
  producer_pid=""
  producer_kind=""
}

reconcile() {
  wanted=placeholder
  if has_consumer; then
    wanted=camera
  fi
  if [ "$wanted" = "$producer_kind" ] && is_producing; then
    return 0
  fi
  if [ "$wanted" = "camera" ]; then
    sleep "$CONSUMER_SETTLE_SECONDS"
    if ! has_consumer; then
      return 0
    fi
  fi
  stop_producer
  if [ "$wanted" = "camera" ]; then
    start_camera_producer
  else
    start_placeholder_producer
  fi
}

if [ ! -c "$SINK_DEVICE" ]; then
  echo "${SINK_DEVICE} is missing - v4l2loopback is not loaded" >&2
  exit 1
fi

sink_side=${SOURCE_VIDEO_SIZE#*x}
producer_pid=""
producer_kind=""

# a consumer cannot negotiate a format until a producer is already streaming, so the sink always carries one
inotifywait --monitor --quiet --event open --event close "$SINK_DEVICE" | { start_placeholder_producer; while read -r -t "$RECONCILE_SECONDS" || [ $? -gt 128 ]; do reconcile; done; }
