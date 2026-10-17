#!/usr/bin/env bash
# Computes release versions from git tags.
#
#   version.sh latest-stable   latest tag vX.Y.Z, printed as X.Y.Z (0.0.0 if none)
#   version.sh next-nightly    version of the next nightly build, e.g. 1.3.3.post2.20261017
#   version.sh has-changes     exit 0 if HEAD is newer than the last nightly, else 1
#
# Nightly rule: take the latest stable version X.Y.Z, bump the minor to X.(Y+1).Z
# and append .postN.YYYYMMDD, where N counts the nightlies already built for
# that base and the date is the UTC day of the build (override with NIGHTLY_DATE).
# Tags like v1.3.3.post1.20261017 are nightlies and never count as stable releases.
set -euo pipefail

STABLE_TAG='^v[0-9]+\.[0-9]+\.[0-9]+$'
NIGHTLY_TAG='^v[0-9]+\.[0-9]+\.[0-9]+\.post[0-9]+\.[0-9]{8}$'

latest_stable() {
  local tag
  tag="$(git tag --list | grep -E "$STABLE_TAG" | sort -V | tail -n 1 || true)"
  echo "${tag#v}"
}

next_nightly() {
  local stable major minor patch base last date
  stable="$(latest_stable)"
  stable="${stable:-0.0.0}"
  IFS=. read -r major minor patch <<<"$stable"
  base="$major.$((minor + 1)).$patch"
  last="$(git tag --list "v$base.post*" | sed -E 's/^v[0-9]+\.[0-9]+\.[0-9]+\.post([0-9]+).*/\1/' | sort -n | tail -n 1 || true)"
  date="${NIGHTLY_DATE:-$(date -u +%Y%m%d)}"
  echo "$base.post$((${last:-0} + 1)).$date"
}

has_changes() {
  local last_tag
  last_tag="$(git tag --list --sort=-creatordate | grep -E "$NIGHTLY_TAG" | head -n 1 || true)"
  [[ -z "$last_tag" ]] && return 0
  [[ "$(git rev-list -n 1 "$last_tag")" != "$(git rev-parse HEAD)" ]]
}

case "${1:-}" in
  latest-stable) latest_stable ;;
  next-nightly) next_nightly ;;
  has-changes) has_changes ;;
  *)
    echo "usage: $0 latest-stable|next-nightly|has-changes" >&2
    exit 2
    ;;
esac
