#!/usr/bin/env bash
# Installs OLD on a running Android emulator or device, launches it, upgrades
# it to NEW in place and checks that the new version starts. This is what a
# user does when they update the app, so a failure here would be a failed update.
#
# With an optional third APK that is signed with a different key, also checks
# that Android rejects it (proving the test would notice a signing mistake).
#
# Usage: adb-upgrade-test.sh OLD.apk NEW.apk [WRONG_KEY.apk]
# Needs: adb with exactly one device or emulator attached, and ANDROID_HOME.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=android-tools.sh
source "$here/android-tools.sh"

old="${1:?usage: $0 OLD.apk NEW.apk [WRONG_KEY.apk]}"
new="${2:?usage: $0 OLD.apk NEW.apk [WRONG_KEY.apk]}"
wrong_key="${3:-}"

pkg="$(apk_field "$old" name)"
old_name="$(apk_field "$old" versionName)"
new_name="$(apk_field "$new" versionName)"
new_code="$(apk_field "$new" versionCode)"

adb wait-for-device
until [[ "$(adb shell getprop sys.boot_completed | tr -d '\r')" == "1" ]]; do sleep 2; done

installed() { # installed FIELD: value of versionName or versionCode of the installed app
  adb shell dumpsys package "$pkg" | tr -d '\r' | sed -n "s/^ *$1=\([^ ]*\).*/\1/p" | head -n 1
}

expect_equal() { # expect_equal DESCRIPTION EXPECTED ACTUAL
  if [[ "$2" == "$3" ]]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1 (expected '$2', got '$3')"
    exit 1
  fi
}

launch_and_check() { # launch_and_check LABEL: start the app and make sure it stays alive
  adb logcat -c
  adb shell monkey -p "$pkg" -c android.intent.category.LAUNCHER 1 >/dev/null
  sleep "${LAUNCH_WAIT:-8}"
  if [[ -z "$(adb shell pidof "$pkg" | tr -d '\r')" ]]; then
    echo "FAIL - $1: the app is not running after launch"
    adb logcat -d | tail -n 40
    exit 1
  fi
  if adb logcat -d -s AndroidRuntime:E | grep -q "FATAL EXCEPTION"; then
    echo "FAIL - $1: the app crashed"
    adb logcat -d -s AndroidRuntime:E | tail -n 30
    exit 1
  fi
  echo "ok   - $1: app launches and keeps running"
}

adb uninstall "$pkg" >/dev/null 2>&1 || true

adb install "$old" >/dev/null
expect_equal "old version $old_name is installed" "$old_name" "$(installed versionName)"
launch_and_check "old version"

adb shell am force-stop "$pkg"
# -r replaces the installed app and keeps its data, like a real update does.
adb install -r "$new" >/dev/null
expect_equal "upgraded to $new_name" "$new_name" "$(installed versionName)"
expect_equal "version code is now $new_code" "$new_code" "$(installed versionCode)"
launch_and_check "new version"

if [[ -n "$wrong_key" ]]; then
  if adb install -r "$wrong_key" >/dev/null 2>&1; then
    echo "FAIL - Android accepted an update signed with a different key"
    exit 1
  fi
  echo "ok   - an update signed with a different key is rejected"
fi

echo "upgrade test passed"
