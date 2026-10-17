#!/usr/bin/env bash
# Tests scripts/version.sh against a throwaway git repository.
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/version.sh"
REPO="$(mktemp -d)"
trap 'rm -rf "$REPO"' EXIT
cd "$REPO"
git init -q -b main
git config user.name test
git config user.email test@example.com
git config commit.gpgsign false
git config tag.gpgsign false

failures=0
expect() { # expect DESCRIPTION EXPECTED ACTUAL
  if [[ "$2" == "$3" ]]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1 (expected '$2', got '$3')"
    failures=$((failures + 1))
  fi
}
commit() { git commit -q --allow-empty -m "$1"; }
changes() { "$SCRIPT" has-changes && echo yes || echo no; }

commit one
expect "no tags: stable is empty" "" "$("$SCRIPT" latest-stable)"
expect "no tags: first nightly" "0.1.0.post1" "$("$SCRIPT" next-nightly)"
expect "no nightly yet counts as changed" yes "$(changes)"

git tag v1.2.3
git tag v1.10.0
git tag v1.9.9
expect "stable uses version sort, not text sort" "1.10.0" "$("$SCRIPT" latest-stable)"

git tag -d v1.10.0 v1.9.9 >/dev/null
expect "nightly bumps the minor and keeps the patch" "1.3.3.post1" "$("$SCRIPT" next-nightly)"

git tag v1.3.3.post1
expect "nightly tags are not stable releases" "1.2.3" "$("$SCRIPT" latest-stable)"
expect "second nightly gets .post2" "1.3.3.post2" "$("$SCRIPT" next-nightly)"
expect "no new commit since the nightly" no "$(changes)"

commit two
expect "new commit since the nightly" yes "$(changes)"

git tag v1.3.3.post2
git tag v1.3.3.post10
expect "post numbers compare numerically" "1.3.3.post11" "$("$SCRIPT" next-nightly)"

git tag v1.3.4
expect "new stable release restarts the nightly numbering" "1.4.4.post1" "$("$SCRIPT" next-nightly)"

((failures == 0)) || exit 1
echo "all version tests passed"
