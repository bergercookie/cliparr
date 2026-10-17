#!/usr/bin/env bash
# Builds an AppImage from a Flutter Linux release bundle.
# Needs `linuxdeploy` and its appimage plugin on PATH, and libmpv installed
# (the player library is bundled into the image).
#
# Usage: build-appimage.sh VERSION BUNDLE_DIR OUTPUT_DIR
set -euo pipefail

version="${1:?version}"
bundle="$(realpath "${2:?path to the Flutter linux bundle}")"
out="$(realpath -m "${3:?output directory}")"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
appdir="$work/AppDir"

install -d "$appdir/usr/lib/cliparr" "$appdir/usr/bin"
cp -r "$bundle"/. "$appdir/usr/lib/cliparr/"
ln -s ../lib/cliparr/cliparr "$appdir/usr/bin/cliparr"

# The Flutter binary looks for its libraries next to itself; the extra
# libraries linuxdeploy collects live in usr/lib, so point the loader there too.
cat >"$work/AppRun" <<'APPRUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
export LD_LIBRARY_PATH="$here/usr/lib:${LD_LIBRARY_PATH:-}"
exec "$here/usr/lib/cliparr/cliparr" "$@"
APPRUN
chmod +x "$work/AppRun"

libmpv="$(ldconfig -p | awk '/libmpv\.so\.2 /{print $NF; exit}')"
[[ -n "$libmpv" ]] || {
  echo "libmpv.so.2 not found; install libmpv2" >&2
  exit 1
}

mkdir -p "$out"
cd "$out"
APPIMAGE_EXTRACT_AND_RUN=1 LINUXDEPLOY_OUTPUT_VERSION="$version" \
  linuxdeploy --appdir "$appdir" \
  --executable "$appdir/usr/lib/cliparr/cliparr" \
  --library "$libmpv" \
  --desktop-file "$here/cliparr.desktop" \
  --icon-file "$here/cliparr.svg" \
  --custom-apprun "$work/AppRun" \
  --output appimage
