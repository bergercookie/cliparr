#!/usr/bin/env bash
# Checks that Android would accept NEW as an in-place update of OLD.
#
# Android refuses an update when the package name or the signing key differs,
# or when the version code does not go up. Those mistakes are easy to make and
# only show up on users' phones, so this catches them before a release.
#
# Usage: check-apk-upgrade.sh OLD.apk NEW.apk
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=android-tools.sh
source "$here/android-tools.sh"

old="${1:?usage: $0 OLD.apk NEW.apk}"
new="${2:?usage: $0 OLD.apk NEW.apk}"
failures=0

fail() {
  echo "FAIL - $1"
  failures=$((failures + 1))
}

old_pkg="$(apk_field "$old" name)"
new_pkg="$(apk_field "$new" name)"
old_code="$(apk_field "$old" versionCode)"
new_code="$(apk_field "$new" versionCode)"
old_name="$(apk_field "$old" versionName)"
new_name="$(apk_field "$new" versionName)"
if [[ -z "$old_pkg$new_pkg$old_code$new_code" ]] || [[ -z "$old_pkg" || -z "$new_pkg" || -z "$old_code" || -z "$new_code" ]]; then
  echo "FAIL - could not read the package name and version code from the APKs" >&2
  exit 1
fi
echo "old: $old_pkg $old_name (code $old_code)"
echo "new: $new_pkg $new_name (code $new_code)"

if [[ "$old_pkg" == "$new_pkg" ]]; then
  echo "ok   - same package name"
else
  fail "package name changed ($old_pkg -> $new_pkg): Android treats it as a different app"
fi

if ((new_code > old_code)); then
  echo "ok   - version code goes up ($old_code -> $new_code)"
else
  fail "version code must increase ($old_code -> $new_code): Android refuses to downgrade"
fi

old_signers="$(apk_signers "$old")"
new_signers="$(apk_signers "$new")"
if [[ -z "$old_signers" || -z "$new_signers" ]]; then
  fail "an APK has no valid signature"
elif [[ "$old_signers" == "$new_signers" ]]; then
  echo "ok   - signed with the same key"
else
  fail "signing key differs: Android refuses the update (users would have to uninstall first)"
fi

old_min="$(apk_min_sdk "$old")"
new_min="$(apk_min_sdk "$new")"
if ((new_min > old_min)); then
  echo "WARN - minimum Android version rises ($old_min -> $new_min): phones on older Android will not get the update"
fi

if ((failures > 0)); then
  echo "$failures upgrade check(s) failed"
  exit 1
fi
echo "upgrade check passed"
