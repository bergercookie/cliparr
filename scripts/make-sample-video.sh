#!/usr/bin/env bash
# Creates a one-minute test video (WebM, with a running timer so every frame
# differs) for the fake Jellyfin. Used by the e2e test and the screenshots.
#
# Usage: make-sample-video.sh OUTPUT_FILE
# Needs: ffmpeg
set -euo pipefail

out="${1:?usage: $0 OUTPUT_FILE}"

command -v ffmpeg >/dev/null || {
  echo "ffmpeg is required to create the sample video." >&2
  exit 1
}

mkdir -p "$(dirname "$out")"
ffmpeg -loglevel error -f lavfi -i "testsrc2=duration=60:size=640x360:rate=15" \
  -c:v libvpx -b:v 300k -y "$out"
