#!/usr/bin/env bash
# Takes screenshots of the Cliparr web app for the documentation.
#
# It starts the server against the fake Jellyfin (with a few sample clips),
# drives a headless Chromium through the main screens and writes PNG files.
#
# Usage: screenshots.sh [OUTPUT_DIR]   (default: docs/screenshots)
# Needs: go, python3, and the web app built into the server (`just build-web`).
# Set CHROMIUM=/path/to/chromium to use an installed browser; otherwise
# Playwright downloads one.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${1:-$root/docs/screenshots}"
jellyfin_port="${SHOTS_JELLYFIN_PORT:-18196}"
cliparr_port="${SHOTS_CLIPARR_PORT:-18180}"

if [[ ! -f "$root/server/internal/web/dist/index.html" ]]; then
  echo "The web app is not built yet. Run 'just build-web' first." >&2
  exit 1
fi

work="$(mktemp -d)"
pids=()
cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$work"
}
trap cleanup EXIT

venv="$root/.venv/screenshots"
python3 -m venv "$venv"
"$venv/bin/pip" install --quiet -r "$root/scripts/requirements-screenshots.txt"
if [[ -z "${CHROMIUM:-}" ]]; then
  "$venv/bin/playwright" install chromium
fi

(cd "$root/server" && go build -o "$work/bin/" ./cmd/cliparr ./cmd/fakejellyfin)
"$work/bin/fakejellyfin" -listen "127.0.0.1:$jellyfin_port" >"$work/jellyfin.log" 2>&1 &
pids+=("$!")
CLIPARR_JELLYFIN_URL="http://127.0.0.1:$jellyfin_port" CLIPARR_LISTEN="127.0.0.1:$cliparr_port" \
  CLIPARR_DATA_DIR="$work/data" "$work/bin/cliparr" >"$work/cliparr.log" 2>&1 &
pids+=("$!")

for _ in $(seq 100); do
  curl -fs -o /dev/null "http://127.0.0.1:$cliparr_port/healthz" && break
  sleep 0.1
done

"$venv/bin/python" "$root/scripts/screenshots.py" "http://127.0.0.1:$cliparr_port" "$out"
