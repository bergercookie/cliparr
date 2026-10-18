#!/usr/bin/env bash
# Tests `just github-release` with a stand-in for the gh program that records how it was called.
#
# The release workflow only ever runs this recipe for real, on a tag, so a mistake in how it
# calls gh (once, the options ended up after `--` and were taken for files) would show up
# only when publishing. This test shows it beforehand.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/bin" "$WORK/dist"
cp "$ROOT/.flutter-version" "$WORK/" # the justfile reads it from its working directory
# A file whose name looks like an option must still be uploaded as a file.
touch "$WORK/dist/cliparr-0.1.0.apk" "$WORK/dist/cliparr_0.1.0_amd64.deb" "$WORK/dist/-odd-name.zip"
cat >"$WORK/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$CALLS"
STUB
chmod +x "$WORK/bin/gh"

failures=0
expect() { # expect DESCRIPTION CONDITION(0 = true)
  if [[ "$2" == 0 ]]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1"
    failures=$((failures + 1))
  fi
}

# release KIND [GITHUB_SHA]: runs the recipe and fills `args` with what gh received.
release() {
  export CALLS="$WORK/calls.txt"
  : >"$CALLS"
  (cd "$WORK" && PATH="$WORK/bin:$PATH" GITHUB_REPOSITORY="owner/repo" GITHUB_SHA="${2:-}" \
    just --justfile "$ROOT/justfile" --working-directory "$WORK" github-release 0.1.0 "$1" >/dev/null 2>&1)
  mapfile -t args <"$CALLS"
}
index_of() { # index_of VALUE: position in args, or -1
  local i
  for i in "${!args[@]}"; do
    if [[ "${args[$i]}" == "$1" ]]; then
      echo "$i"
      return
    fi
  done
  echo -1
}

release stable
expect "creates the release for the tag" "$([[ "${args[0]} ${args[1]} ${args[2]}" == "release create v0.1.0" ]] && echo 0 || echo 1)"
separator="$(index_of --)"
expect "separates the files from the options with --" "$([[ "$separator" -gt 0 ]] && echo 0 || echo 1)"
for option in --repo --title --generate-notes; do
  position="$(index_of "$option")"
  expect "$option is an option, before the --" "$([[ "$position" -gt 2 && "$position" -lt "$separator" ]] && echo 0 || echo 1)"
done
expect "the repository is the one given" "$([[ "${args[$(($(index_of --repo) + 1))]}" == "owner/repo" ]] && echo 0 || echo 1)"
files=("${args[@]:$((separator + 1))}")
expect "uploads the three files and the checksums, nothing else" \
  "$([[ "${files[*]}" == "-odd-name.zip SHA256SUMS cliparr-0.1.0.apk cliparr_0.1.0_amd64.deb" ]] && echo 0 || echo 1)"
expect "writes a checksum line per file" "$([[ "$(wc -l <"$WORK/dist/SHA256SUMS")" -eq 3 ]] && echo 0 || echo 1)"
expect "a stable release is not marked as a prerelease" "$([[ "$(index_of --prerelease)" -eq -1 ]] && echo 0 || echo 1)"
expect "without GITHUB_SHA the tag is not pinned" "$([[ "$(index_of --target)" -eq -1 ]] && echo 0 || echo 1)"

rm -f "$WORK/dist/SHA256SUMS"
release prerelease abc123
position="$(index_of --prerelease)"
expect "a nightly is marked as a prerelease, before the --" "$([[ "$position" -gt 2 && "$position" -lt "$(index_of --)" ]] && echo 0 || echo 1)"
position="$(index_of --target)"
expect "a nightly is pinned to the built commit" \
  "$([[ "$position" -gt 2 && "$position" -lt "$(index_of --)" && "${args[$((position + 1))]}" == "abc123" ]] && echo 0 || echo 1)"

if ((failures)); then
  echo "$failures check(s) failed"
  exit 1
fi
