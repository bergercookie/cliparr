#!/usr/bin/env bash
# End-to-end test: starts the real Cliparr binary against a fake Jellyfin and
# checks both login modes:
#   1. direct password login (what the Android app uses)
#   2. SSO: Caddy + (fake) Authelia in front of Cliparr, as in docs/sso-authelia.md
#
# Requires: go, curl, jq, caddy.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
PIDS=()
FAILURES=0

export E2E_JELLYFIN_PORT="${E2E_JELLYFIN_PORT:-18096}"
export E2E_AUTHELIA_PORT="${E2E_AUTHELIA_PORT:-19091}"
export E2E_CLIPARR_PORT="${E2E_CLIPARR_PORT:-18080}"
export E2E_UNTRUSTED_PORT="${E2E_UNTRUSTED_PORT:-18081}"
export E2E_CADDY_PORT="${E2E_CADDY_PORT:-18443}"
export E2E_APP_PORT="${E2E_APP_PORT:-18444}"

DIRECT="http://127.0.0.1:$E2E_CLIPARR_PORT"
UNTRUSTED="http://127.0.0.1:$E2E_UNTRUSTED_PORT"
VIA_PROXY="http://127.0.0.1:$E2E_CADDY_PORT"
APP_HOST="http://127.0.0.1:$E2E_APP_PORT"
AUTHELIA="http://127.0.0.1:$E2E_AUTHELIA_PORT"

cleanup() {
  for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT

start() { # start NAME COMMAND... : run in the background, logging to $WORK/NAME.log
  local name="$1"
  shift
  "$@" >"$WORK/$name.log" 2>&1 &
  PIDS+=("$!")
}

wait_for() { # wait_for URL
  for _ in $(seq 100); do
    curl -fs -o /dev/null "$1" && return 0
    sleep 0.1
  done
  echo "timed out waiting for $1" >&2
  cat "$WORK"/*.log >&2
  exit 1
}

check() { # check DESCRIPTION EXPECTED ACTUAL
  if [[ "$2" == "$3" ]]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1 (expected '$2', got '$3')"
    FAILURES=$((FAILURES + 1))
  fi
}

status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
json() { curl -fs "$@"; }

echo "building..."
(cd "$ROOT/server" && go build -o "$WORK/bin/" ./cmd/...)

start jellyfin "$WORK/bin/fakejellyfin" -listen "127.0.0.1:$E2E_JELLYFIN_PORT"
start authelia "$WORK/bin/fakeauthelia" -listen "127.0.0.1:$E2E_AUTHELIA_PORT"

cliparr() { # cliparr NAME PORT TRUSTED_NETWORK
  CLIPARR_JELLYFIN_URL="http://127.0.0.1:$E2E_JELLYFIN_PORT" \
    CLIPARR_LISTEN="127.0.0.1:$2" \
    CLIPARR_DATA_DIR="$WORK/data-$1" \
    CLIPARR_PROXY_AUTH_HEADER="Remote-User" \
    CLIPARR_PROXY_AUTH_TRUSTED="$3" \
    CLIPARR_JELLYFIN_API_KEY="test-api-key" \
    start "$1" "$WORK/bin/cliparr"
}
# Same server twice: once trusting the local proxy, once trusting some other network.
cliparr cliparr "$E2E_CLIPARR_PORT" "127.0.0.1/32"
cliparr untrusted "$E2E_UNTRUSTED_PORT" "10.255.255.0/24"
start caddy caddy run --config "$ROOT/e2e/Caddyfile" --adapter caddyfile

wait_for "$DIRECT/healthz"
wait_for "$UNTRUSTED/healthz"
wait_for "$AUTHELIA/login?user=probe"
# Through the proxy every request needs a login, so Caddy is ready once it answers 401.
for _ in $(seq 100); do
  [[ "$(status "$VIA_PROXY/api/me")" == 401 ]] && break
  sleep 0.1
done

echo
echo "== password login (apps, no SSO) =="
check "wrong password is rejected" 401 \
  "$(status -X POST "$DIRECT/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"alice","password":"nope"}')"
TOKEN="$(json -X POST "$DIRECT/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}' | jq -r .token)"
check "login returns a token" true "$([[ -n "$TOKEN" && "$TOKEN" != null ]] && echo true || echo false)"
check "token identifies alice" alice "$(json "$DIRECT/api/me" -H "Authorization: Bearer $TOKEN" | jq -r .name)"
check "password login is not 'via proxy'" false \
  "$(json "$DIRECT/api/me" -H "Authorization: Bearer $TOKEN" | jq -r .viaProxy)"
CLIP="$(json -X POST "$DIRECT/api/clips" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"itemId":"movie-rocky","startMs":1000,"endMs":9000,"quote":"Yo Adrian!","tags":["classic"]}')"
check "clip gets the title from Jellyfin" "Rocky (1976)" "$(jq -r .title <<<"$CLIP")"
check "stream works with range requests" 206 \
  "$(status "$DIRECT/api/items/movie-rocky/stream" -H "Authorization: Bearer $TOKEN" -H 'Range: bytes=0-9')"

echo
echo "== SSO through Caddy + Authelia =="
check "anonymous request is stopped by the proxy" 401 "$(status "$VIA_PROXY/api/me")"

JAR_ALICE="$WORK/alice.jar"
JAR_BOB="$WORK/bob.jar"
curl -fs -c "$JAR_ALICE" "$AUTHELIA/login?user=alice"
curl -fs -c "$JAR_BOB" "$AUTHELIA/login?user=bob"
ME="$(json -b "$JAR_ALICE" "$VIA_PROXY/api/me")"
check "SSO user alice is recognised without a Jellyfin password" alice "$(jq -r .name <<<"$ME")"
check "SSO login is marked 'via proxy'" true "$(jq -r .viaProxy <<<"$ME")"
check "client cannot override the user header" alice \
  "$(json -b "$JAR_ALICE" -H 'Remote-User: bob' "$VIA_PROXY/api/me" | jq -r .name)"

check "SSO alice sees her password-login clip" 1 "$(json -b "$JAR_ALICE" "$VIA_PROXY/api/clips" | jq length)"
check "SSO bob does not" 0 "$(json -b "$JAR_BOB" "$VIA_PROXY/api/clips" | jq length)"
check "SSO alice can create a clip" 201 \
  "$(status -b "$JAR_ALICE" -X POST "$VIA_PROXY/api/clips" -H 'Content-Type: application/json' \
    -d '{"itemId":"movie-secret","startMs":0,"endMs":5000}')"

check "SSO alice can stream her private movie" 200 "$(status -b "$JAR_ALICE" "$VIA_PROXY/api/items/movie-secret/stream")"
check "SSO bob cannot stream it (API key is limited to his view)" 404 \
  "$(status -b "$JAR_BOB" "$VIA_PROXY/api/items/movie-secret/stream")"
check "SSO bob cannot clip it" 404 \
  "$(status -b "$JAR_BOB" -X POST "$VIA_PROXY/api/clips" -H 'Content-Type: application/json' \
    -d '{"itemId":"movie-secret","startMs":0,"endMs":5000}')"

curl -fs -c "$WORK/mallory.jar" "$AUTHELIA/login?user=mallory"
check "SSO user without a Jellyfin account is refused" 403 "$(status -b "$WORK/mallory.jar" "$VIA_PROXY/api/me")"

echo
echo "== header spoofing =="
check "header is ignored when the sender is not a trusted proxy" 401 \
  "$(status -H 'Remote-User: alice' "$UNTRUSTED/api/me")"
check "password login still works on that server" 200 \
  "$(status "$UNTRUSTED/api/me" -H "Authorization: Bearer $(json -X POST "$UNTRUSTED/api/auth/login" \
    -H 'Content-Type: application/json' -d '{"username":"bob","password":"pw"}' | jq -r .token)")"

check "header cannot be smuggled through the proxy's app hostname" 401 \
  "$(status -H 'Remote-User: alice' "$APP_HOST/api/me")"
APP_TOKEN="$(json -X POST "$APP_HOST/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}' | jq -r .token)"
check "apps can still log in with their Jellyfin password via that hostname" alice \
  "$(json "$APP_HOST/api/me" -H "Authorization: Bearer $APP_TOKEN" | jq -r .name)"

echo
if ((FAILURES > 0)); then
  echo "$FAILURES check(s) failed"
  exit 1
fi
echo "all e2e checks passed"
