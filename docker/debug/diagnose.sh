#!/usr/bin/env bash
# Checks that Cliparr and Jellyfin are reachable from this container.
#
# Usage: cliparr-diagnose [CLIPARR_URL] [JELLYFIN_URL]
# Defaults come from CLIPARR_URL / JELLYFIN_URL, then http://cliparr:8080 and http://jellyfin:8096.
set -uo pipefail

cliparr_url="${1:-${CLIPARR_URL:-http://cliparr:8080}}"
jellyfin_url="${2:-${JELLYFIN_URL:-http://jellyfin:8096}}"
failed=0

step() { # step DESCRIPTION COMMAND...
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "ok   - $description"
  else
    echo "FAIL - $description"
    failed=1
  fi
}

host_of() { sed -E 's#^[a-z]+://([^/:]+).*#\1#' <<<"$1"; }

step "DNS resolves $(host_of "$cliparr_url")" getent hosts "$(host_of "$cliparr_url")"
step "Cliparr health check ($cliparr_url/healthz)" curl -fsS -m 5 "$cliparr_url/healthz"
step "Cliparr identifies itself ($cliparr_url/api/info)" curl -fsS -m 5 "$cliparr_url/api/info"
step "DNS resolves $(host_of "$jellyfin_url")" getent hosts "$(host_of "$jellyfin_url")"
step "Jellyfin is reachable ($jellyfin_url/System/Info/Public)" curl -fsS -m 5 "$jellyfin_url/System/Info/Public"

if ((failed)); then
  echo
  echo "Something failed. Re-run a failing command without the silencing to see the error, e.g.:"
  echo "  curl -v $cliparr_url/healthz"
  exit 1
fi
echo "Everything looks fine."
