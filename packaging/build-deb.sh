#!/usr/bin/env bash
# Builds a .deb from a Flutter Linux release bundle.
#
# Usage: build-deb.sh VERSION BUNDLE_DIR OUTPUT_DIR
set -euo pipefail

version="${1:?version}"
bundle="${2:?path to the Flutter linux bundle}"
out="${3:?output directory}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

install -d "$root/opt/cliparr" "$root/usr/bin" "$root/usr/share/applications" \
  "$root/usr/share/icons/hicolor/scalable/apps" "$root/DEBIAN"
cp -r "$bundle"/. "$root/opt/cliparr/"
ln -s /opt/cliparr/cliparr "$root/usr/bin/cliparr"
install -m 644 "$here/cliparr.desktop" "$root/usr/share/applications/cliparr.desktop"
install -m 644 "$here/cliparr.svg" "$root/usr/share/icons/hicolor/scalable/apps/cliparr.svg"

cat >"$root/DEBIAN/control" <<CONTROL
Package: cliparr
Version: $version
Architecture: amd64
Maintainer: Nikos Koukis <nickkouk@gmail.com>
Depends: libgtk-3-0, libmpv2
Section: video
Priority: optional
Homepage: https://github.com/bergercookie/cliparr
Description: Save and replay your favorite Jellyfin moments
 Desktop client for a Cliparr server. Enter your server address on first start.
CONTROL

mkdir -p "$out"
dpkg-deb --root-owner-group --build "$root" "$out/cliparr_${version}_amd64.deb"
