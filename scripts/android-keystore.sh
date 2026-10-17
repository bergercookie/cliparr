#!/usr/bin/env bash
# Creates the key that signs Cliparr's Android releases.
#
# Android only installs an APK as an update if it is signed with the same key
# as the installed app. Create this key once, keep it forever, and store it in
# your GitHub repository secrets. Losing it means every user has to uninstall
# the app before they can install a new release.
#
# Usage: android-keystore.sh [OUTPUT_DIR]   (default: build/signing)
set -euo pipefail

out="${1:-build/signing}"
keystore="$out/cliparr-release.jks"

if [[ -e "$keystore" ]]; then
  echo "$keystore already exists. Refusing to overwrite a signing key." >&2
  exit 1
fi

mkdir -p "$out"
chmod 700 "$out"

password="$(openssl rand -hex 16)"
keytool -genkeypair -keystore "$keystore" -storetype PKCS12 \
  -alias cliparr -keyalg RSA -keysize 4096 -validity 36500 \
  -storepass "$password" -keypass "$password" \
  -dname "CN=Cliparr" >/dev/null

# One file per secret, so no password ever appears in your terminal history.
base64 -w0 "$keystore" >"$out/keystore.base64"
printf '%s' "$password" >"$out/password.txt"
printf '%s' cliparr >"$out/alias.txt"
chmod 600 "$out"/*

cat <<MESSAGE
Created $keystore

1. BACK UP the whole $out directory somewhere safe (a password manager works).
2. Store the four secrets in GitHub (needs the gh CLI, logged in):

   gh secret set ANDROID_KEYSTORE_BASE64   < $out/keystore.base64
   gh secret set ANDROID_KEYSTORE_PASSWORD < $out/password.txt
   gh secret set ANDROID_KEY_PASSWORD      < $out/password.txt
   gh secret set ANDROID_KEY_ALIAS         < $out/alias.txt

3. Never commit this directory. (build/ is already git-ignored.)
MESSAGE
