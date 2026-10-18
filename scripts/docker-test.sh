#!/usr/bin/env bash
# Tests who owns what in the data folder, with the real Docker image.
#
# People mount a folder of their own as /data and want everything Cliparr writes there to
# belong to them, so they can back it up. This starts the image in the situations that
# matter, as the current user, and reads the owners back from the host.
#
# Usage: docker-test.sh [IMAGE]   (default: cliparr:dev, made by `just docker-build`)
# Needs: docker, and a user who can use it.
set -euo pipefail

image="${1:-cliparr:dev}"
uid="$(id -u)"
gid="$(id -g)"
work="$(mktemp -d)"
tag="cliparr-docker-test-$$"
containers=()

cleanup() {
  for name in "${containers[@]}"; do docker rm -f "$name" >/dev/null 2>&1 || true; done
  docker volume rm "$tag" >/dev/null 2>&1 || true
  # Some files belong to other users, so let a container (root) remove them.
  docker run --rm -v "$work:/work" --entrypoint sh "$image" -c 'rm -rf /work/* /work/.[!.]*' >/dev/null 2>&1 || true
  rmdir "$work" 2>/dev/null || true
}
trap cleanup EXIT

failures=0
expect() { # expect DESCRIPTION EXPECTED ACTUAL
  if [[ "$2" == "$3" ]]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1 (expected '$2', got '$3')"
    failures=$((failures + 1))
  fi
}

# start NAME DOCKER_ARGS...: runs Cliparr and waits until it has started or stopped.
start() {
  local name="$tag-$1"
  shift
  containers+=("$name")
  docker run -d --name "$name" --network none -e CLIPARR_JELLYFIN_URL=http://jellyfin:8096 "$@" "$image" >/dev/null
  for _ in $(seq 100); do
    docker logs "$name" 2>&1 | grep -q 'msg="cliparr started"' && return 0
    [[ "$(docker inspect -f '{{.State.Running}}' "$name")" == true ]] || return 0
    sleep 0.1
  done
}
owner() { stat -c %u:%g "$1" 2>/dev/null || echo missing; }
running() { docker inspect -f '{{.State.Running}}' "$tag-$1"; }
# who NAME: user and group of the Cliparr process (real, effective, saved, file system ids)
who() {
  docker exec "$tag-$1" awk '/^Uid:/ {u = $2 "," $3 "," $4 "," $5} /^Gid:/ {g = $2 "," $3 "," $4 "," $5} END {print u " " g}' /proc/1/status
}

echo "== a folder of your own, PUID and PGID set to you"
mkdir "$work/own"
start own -v "$work/own:/data" -e PUID="$uid" -e PGID="$gid"
expect "it starts" true "$(running own)"
expect "the database belongs to you" "$uid:$gid" "$(owner "$work/own/cliparr.db")"
expect "so does the folder" "$uid:$gid" "$(owner "$work/own")"
expect "Cliparr runs as you, with no way back to root" "$uid,$uid,$uid,$uid $gid,$gid,$gid,$gid" "$(who own)"

echo "== a folder that does not exist yet: Docker creates it as root"
start created -v "$work/created:/data" -e PUID="$uid" -e PGID="$gid"
expect "it starts" true "$(running created)"
expect "the folder was handed to you" "$uid:$gid" "$(owner "$work/created")"
expect "and the database in it" "$uid:$gid" "$(owner "$work/created/cliparr.db")"

echo "== leftovers of an earlier run as another user, in subfolders too"
mkdir -p "$work/old/thumbnails"
docker run --rm -v "$work/old:/data" --entrypoint sh "$image" -c 'touch /data/cliparr.db /data/thumbnails/a.jpg && chown -R 65532:65532 /data'
start old -v "$work/old:/data" -e PUID="$uid" -e PGID="$gid"
expect "it starts" true "$(running old)"
expect "the old database was handed to you" "$uid:$gid" "$(owner "$work/old/cliparr.db")"
expect "and a file in a subfolder" "$uid:$gid" "$(owner "$work/old/thumbnails/a.jpg")"

echo "== a link inside the folder must not change files outside it"
mkdir -p "$work/links" "$work/outside"
touch "$work/outside/precious"
ln -s /outside/precious "$work/links/link-to-precious"
before="$(owner "$work/outside/precious")"
start links -v "$work/links:/data" -v "$work/outside:/outside" -e PUID="$((uid + 1))" -e PGID="$((gid + 1))"
expect "it starts" true "$(running links)"
expect "the file the link points to keeps its owner" "$before" "$(owner "$work/outside/precious")"

echo "== PUID and PGID left out"
docker volume create "$tag" >/dev/null
start default -v "$tag:/data"
expect "it starts" true "$(running default)"
expect "the database belongs to the image's own user, 65532" "65532:65532" \
  "$(docker run --rm -v "$tag:/data" --entrypoint stat "$image" -c %u:%g /data/cliparr.db)"

echo "== root is refused"
start root -v "$work/own:/data" -e PUID=0
expect "it does not start" false "$(running root)"
expect "and says why" yes "$(docker logs "$tag-root" 2>&1 | grep -q 'which is root' && echo yes || echo no)"

echo "== user: instead, so that it is never root"
mkdir "$work/useronly"
start useronly -v "$work/useronly:/data" --user "$uid:$gid"
expect "it starts" true "$(running useronly)"
expect "the database belongs to you" "$uid:$gid" "$(owner "$work/useronly/cliparr.db")"
expect "Cliparr runs as you" "$uid,$uid,$uid,$uid $gid,$gid,$gid,$gid" "$(who useronly)"

echo "== user: with a folder Docker made as root"
start useronly-root -v "$work/useronly-root:/data" --user "$uid:$gid"
expect "it does not start" false "$(running useronly-root)"
expect "and tells you what to do" yes "$(docker logs "$tag-useronly-root" 2>&1 | grep -q 'not writable' && echo yes || echo no)"

if ((failures)); then
  echo
  echo "$failures check(s) failed"
  exit 1
fi
