#!/usr/bin/env bash
# End-to-end test against a real Jellyfin server: downloads a release from
# repo.jellyfin.org, starts it with a fresh database and a generated test movie,
# starts Cliparr, then runs the checks in e2e/jellyfin/:
#   server_checks     login, search, streaming, thumbnails, who sees what, resume point
#   web_checks        "open in Jellyfin web" and "play on a device" in Jellyfin's web client
#   extension_checks  the browser extension on Jellyfin's real player
#
# Usage: jellyfin-e2e.sh [VERSION]
#   VERSION  a Jellyfin release such as 10.11.11 or 12.2, or latest-stable (default).
#            Only the releases that repo.jellyfin.org still offers can be tested.
# Needs: go, python3, jq, zip, curl, ffmpeg. Set CHROMIUM=/path/to/chromium to use
# an installed browser; otherwise Playwright downloads one. The download is kept in
# build/jellyfin/ and reused. Set JELLYFIN_E2E_KEEP=1 to keep the temporary directory
# (Jellyfin's data and log) for debugging.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-latest-stable}"
jellyfin_port="${JELLYFIN_E2E_PORT:-18096}"
cliparr_port="${CLIPARR_E2E_PORT:-18180}"
repo="https://repo.jellyfin.org/files/server/linux"

if [[ "$version" == "latest-stable" ]]; then
  dir="$repo/latest-stable/amd64"
  file="$(curl -fsSL "$dir/" | grep -oE 'jellyfin_[0-9.]+-amd64\.tar\.gz' | head -n 1)"
  [[ -n "$file" ]] || {
    echo "Could not find the latest Jellyfin release in $dir" >&2
    exit 1
  }
else
  dir="$repo/stable/v$version/amd64"
  file="jellyfin_$version-amd64.tar.gz"
fi

mkdir -p "$root/build/jellyfin"
tarball="$root/build/jellyfin/$file"
if [[ ! -s "$tarball" ]]; then
  echo "Downloading $file"
  curl -fSL --retry 3 -o "$tarball.part" "$dir/$file"
  mv "$tarball.part" "$tarball"
fi

work="$(mktemp -d)"
pids=()
cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  # Let Jellyfin stop its ffmpeg transcodes before their folder disappears.
  for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
  if [[ -n "${JELLYFIN_E2E_KEEP:-}" ]]; then
    echo "Kept $work (Jellyfin's log is $work/jellyfin.log)"
  else
    rm -rf "$work"
  fi
}
trap cleanup EXIT

mkdir -p "$work/server" "$work/media/Harbor Lights (2020)"
tar -xzf "$tarball" -C "$work/server"

# Ten minutes of H.264/AAC with a running timer, small and quick to make. The movie is
# long enough for Jellyfin to offer resuming it, and the timer shows where a player is.
ffmpeg -loglevel error -f lavfi -i "testsrc2=duration=600:size=320x180:rate=12" \
  -f lavfi -i "sine=frequency=330:duration=600" -c:v libx264 -pix_fmt yuv420p -preset ultrafast -crf 35 \
  -c:a aac -b:a 32k -shortest -movflags +faststart "$work/media/Harbor Lights (2020)/Harbor Lights (2020).mp4"

jellyfin_url="http://127.0.0.1:$jellyfin_port"
cliparr_url="http://127.0.0.1:$cliparr_port"

# Jellyfin has no command-line option for its port: it is read from network.xml.
mkdir -p "$work/jf/config"
cat >"$work/jf/config/network.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<NetworkConfiguration>
  <InternalHttpPort>$jellyfin_port</InternalHttpPort>
  <PublicHttpPort>$jellyfin_port</PublicHttpPort>
  <AutoDiscovery>false</AutoDiscovery>
  <EnableUPnP>false</EnableUPnP>
</NetworkConfiguration>
XML

"$work/server/jellyfin/jellyfin" --datadir "$work/jf/data" --configdir "$work/jf/config" --cachedir "$work/jf/cache" \
  --logdir "$work/jf/log" --ffmpeg "$(command -v ffmpeg)" >"$work/jellyfin.log" 2>&1 &
pids+=("$!")
for _ in $(seq 300); do
  curl -fs -o /dev/null "$jellyfin_url/System/Info/Public" 2>/dev/null && break
  sleep 0.2
done
curl -fs -o /dev/null "$jellyfin_url/System/Info/Public" || {
  echo "Jellyfin did not start. Its log:" >&2
  tail -n 30 "$work/jellyfin.log" >&2
  exit 1
}
jellyfin_version="$(curl -fs "$jellyfin_url/System/Info/Public" | jq -r .version)"
echo "Jellyfin $jellyfin_version is running on port $jellyfin_port"

# shellcheck source=browser-env.sh
source "$root/scripts/browser-env.sh"
python="$(browser_python)"

(cd "$root/server" && go build -o "$work/cliparr" ./cmd/cliparr)
"$root/scripts/build-extension.sh" 0.0.0 --test-hosts "http://127.0.0.1/*" >/dev/null

"$python" "$root/e2e/jellyfin/setup.py" "$jellyfin_url" "$work/media" || {
  echo "Setting up Jellyfin failed. The end of its log:" >&2
  tail -n 25 "$work/jellyfin.log" >&2
  exit 1
}

CLIPARR_JELLYFIN_URL="$jellyfin_url" CLIPARR_LISTEN="127.0.0.1:$cliparr_port" CLIPARR_DATA_DIR="$work/cliparr-data" \
  "$work/cliparr" >"$work/cliparr.log" 2>&1 &
pids+=("$!")
for _ in $(seq 100); do
  curl -fs -o /dev/null "$cliparr_url/healthz" && break
  sleep 0.1
done

# On startup Cliparr checks Jellyfin and logs its version.
for _ in $(seq 100); do
  grep -q 'msg="jellyfin reachable"' "$work/cliparr.log" && break
  sleep 0.1
done
if grep 'msg="jellyfin reachable"' "$work/cliparr.log" | grep -qF "version=$jellyfin_version"; then
  echo "Cliparr's log says Jellyfin $jellyfin_version is reachable"
else
  echo "Cliparr's log does not report Jellyfin $jellyfin_version as reachable:" >&2
  cat "$work/cliparr.log" >&2
  exit 1
fi

status=0
for check in server_checks web_checks; do
  echo
  echo "##### $check"
  "$python" "$root/e2e/jellyfin/$check.py" "$cliparr_url" "$jellyfin_url" || status=1
done
echo
echo "##### extension_checks"
"$python" "$root/e2e/jellyfin/extension_checks.py" "$root/build/extension/chrome-test" "$cliparr_url" "$jellyfin_url" || status=1

if ((status)); then
  echo >&2
  echo "Something failed. The end of Jellyfin's log:" >&2
  tail -n 15 "$work/jellyfin.log" >&2
fi
((status == 0)) # a failed check makes the script, and so CI, fail
