# shellcheck shell=bash
# Shared helpers for the Android scripts. Source this file; it is not run directly.
# Needs ANDROID_HOME (or ANDROID_SDK_ROOT) pointing at an Android SDK with build-tools.

sdk_dir="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"

# build_tool NAME: path of the newest build-tools binary, e.g. aapt or apksigner.
build_tool() {
  local path
  path="$(find "$sdk_dir/build-tools" -mindepth 2 -maxdepth 2 -name "$1" 2>/dev/null | sort -V | tail -n 1)"
  if [[ -z "$path" ]]; then
    echo "$1 not found: set ANDROID_HOME to an Android SDK with build-tools installed." >&2
    exit 1
  fi
  echo "$path"
}

# apk_field APK FIELD: value of package name, versionCode or versionName.
apk_field() {
  "$(build_tool aapt)" dump badging "$1" | sed -n "s/^package:.*[[:space:]]$2='\([^']*\)'.*/\1/p"
}

# apk_min_sdk APK: lowest Android API level the APK installs on.
apk_min_sdk() {
  "$(build_tool aapt)" dump badging "$1" | sed -n "s/^sdkVersion:'\([0-9]*\)'.*/\1/p"
}

# apk_signers APK: SHA-256 digests of the signing certificates, sorted. Fails,
# printing what apksigner said, if the signature is broken or none is found.
apk_signers() {
  local output digests
  if ! output="$("$(build_tool apksigner)" verify --print-certs "$1" 2>&1)"; then
    echo "apksigner could not verify $1:" >&2
    echo "$output" >&2
    return 1
  fi
  # The line prefix differs between apksigner versions ("Signer #1 certificate",
  # "V2 Signer: certificate", ...) and the same key can be listed once per
  # signature scheme, so match on the digest itself and drop duplicates.
  digests="$(sed -n 's/^.*certificate SHA-256 digest: //p' <<<"$output" | sort -u)"
  if [[ -z "$digests" ]]; then
    echo "no signing certificate found in $1; apksigner said:" >&2
    echo "$output" >&2
    return 1
  fi
  echo "$digests"
}
