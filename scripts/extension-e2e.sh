#!/usr/bin/env bash
# End-to-end test of the browser extension: builds it, starts the fake Jellyfin
# (with a real video and a web player page) and Cliparr, and drives Chromium.
#
# Needs: go, python3, jq, zip, ffmpeg. Set CHROMIUM=/path/to/chromium to use an
# installed browser; otherwise Playwright downloads one.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
jellyfin_port="${EXT_JELLYFIN_PORT:-18296}"
cliparr_port="${EXT_CLIPARR_PORT:-18280}"

work="$(mktemp -d)"
pids=()
cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$work"
}
trap cleanup EXIT

# shellcheck source=browser-env.sh
source "$root/scripts/browser-env.sh"
python="$(browser_python)"

"$root/scripts/build-extension.sh" 0.0.0 --test-hosts "http://127.0.0.1/*,https://127.0.0.1/*"
"$root/scripts/make-sample-video.sh" "$work/sample.webm"

(cd "$root/server" && go build -o "$work/bin/" ./cmd/cliparr ./cmd/fakejellyfin)
"$work/bin/fakejellyfin" -listen "127.0.0.1:$jellyfin_port" -video "$work/sample.webm" >"$work/jellyfin.log" 2>&1 &
pids+=("$!")
CLIPARR_JELLYFIN_URL="http://127.0.0.1:$jellyfin_port" CLIPARR_LISTEN="127.0.0.1:$cliparr_port" \
  CLIPARR_DATA_DIR="$work/data" "$work/bin/cliparr" >"$work/cliparr.log" 2>&1 &
pids+=("$!")

for _ in $(seq 100); do
  curl -fs -o /dev/null "http://127.0.0.1:$cliparr_port/healthz" && break
  sleep 0.1
done

"$python" "$root/extension/e2e/e2e.py" "$root/build/extension/chrome-test" \
  "http://127.0.0.1:$cliparr_port" "http://127.0.0.1:$jellyfin_port"
