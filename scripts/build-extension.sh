#!/usr/bin/env bash
# Builds the browser extension for Chrome and Firefox from extension/.
#
# Both browsers share the code and differ only in the manifest: Chrome runs the
# background script as a service worker, Firefox as an event page.
#
# Usage: build-extension.sh [VERSION] [--test-hosts PATTERN]
#   VERSION         e.g. 1.2.3 or 1.3.3.post2.20261017 (default: 0.0.0)
#   --test-hosts    build only a Chrome copy for the end-to-end test, with access
#                   to the given pages (e.g. "http://127.0.0.1/*") granted at
#                   install, because a test cannot click the permission prompt
#
# Output: build/extension/chrome and build/extension/firefox, plus zip files in
# dist/ (not for --test-hosts).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$root/extension"
out="$root/build/extension"

version="0.0.0"
test_hosts=""
while (($#)); do
  case "$1" in
    --test-hosts)
      test_hosts="${2:?--test-hosts needs a pattern}"
      shift 2
      ;;
    *)
      version="$1"
      shift
      ;;
  esac
done

# Browsers want up to four dotted numbers: 1.3.3.post2.20261017 becomes 1.3.3.2.
base="${version%%.post*}"
[[ "$base" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || base="0.0.0"
manifest_version="$base"
if [[ "$version" =~ \.post([0-9]+) ]]; then
  manifest_version="$base.${BASH_REMATCH[1]}"
fi

FILES=(lib.js background.js content.js options.html options.js icons)

# The jq filters below use $name and $hosts on purpose: jq fills them in, not the shell.
# shellcheck disable=SC2016
# assemble DIR JQ_FILTER: copy the files and write the manifest.
assemble() {
  local dir="$1" filter="$2"
  rm -rf "$dir"
  mkdir -p "$dir"
  for f in "${FILES[@]}"; do cp -r "$src/$f" "$dir/"; done
  jq --arg version "$manifest_version" --arg name "$version" --argjson hosts "$(jq -nc --arg h "$test_hosts" '[$h]')" \
    ".version = \$version | $filter" "$src/manifest.base.json" >"$dir/manifest.json"
}

# shellcheck disable=SC2016
if [[ -n "$test_hosts" ]]; then
  assemble "$out/chrome-test" '.background = {service_worker: "background.js"} | .host_permissions = $hosts'
  echo "Built $out/chrome-test"
  exit 0
fi

# shellcheck disable=SC2016
assemble "$out/chrome" '.version_name = $name | .background = {service_worker: "background.js"}'
assemble "$out/firefox" '.background = {scripts: ["lib.js", "background.js"]}
  | .browser_specific_settings = {
      gecko: {
        id: "cliparr@bergercookie.github.io",
        strict_min_version: "140.0",
        data_collection_permissions: {required: ["none"]}
      },
      gecko_android: {strict_min_version: "142.0"}
    }'

mkdir -p "$root/dist"
for browser in chrome firefox; do
  zip_file="$root/dist/cliparr-extension-$version-$browser.zip"
  rm -f "$zip_file"
  (cd "$out/$browser" && zip -qr "$zip_file" .)
  echo "Built $zip_file"
done
