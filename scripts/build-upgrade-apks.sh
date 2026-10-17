#!/usr/bin/env bash
# Builds the APKs used by the upgrade test: an "old" and a "new" version signed
# with the same throwaway key, plus a copy of "new" signed with another key.
# Nothing here uses the real release key.
#
# Usage: build-upgrade-apks.sh [OUTPUT_DIR]   (default: build/upgrade-test)
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=android-tools.sh
source "$here/android-tools.sh"

out="$(realpath -m "${1:-build/upgrade-test}")"
mkdir -p "$out"
password="upgrade-test" # throwaway: these keys only exist for this test

make_key() { # make_key NAME
  rm -f "$out/$1.jks"
  keytool -genkeypair -keystore "$out/$1.jks" -storetype PKCS12 -alias cliparr \
    -keyalg RSA -keysize 2048 -validity 30 -storepass "$password" -keypass "$password" \
    -dname "CN=Cliparr upgrade test $1" >/dev/null 2>&1
}

build_apk() { # build_apk VERSION BUILD_NUMBER OUTPUT_FILE
  (
    cd "$here/../app"
    CLIPARR_KEYSTORE_FILE="$out/key-a.jks" CLIPARR_KEYSTORE_PASSWORD="$password" \
      CLIPARR_KEY_PASSWORD="$password" CLIPARR_KEY_ALIAS=cliparr \
      flutter build apk --release --dart-define=CLIPARR_VERSION="$1" \
      --build-name "$1" --build-number "$2"
  )
  cp "$here/../app/build/app/outputs/flutter-apk/app-release.apk" "$3"
}

make_key key-a
make_key key-b
build_apk 0.0.1 1 "$out/old.apk"
build_apk 0.0.2 2 "$out/new.apk"

"$(build_tool apksigner)" sign --v4-signing-enabled false --ks "$out/key-b.jks" --ks-pass "pass:$password" \
  --key-pass "pass:$password" --out "$out/wrong-key.apk" "$out/new.apk"

echo "Built $out/old.apk, new.apk and wrong-key.apk"
