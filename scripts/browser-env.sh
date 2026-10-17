# shellcheck shell=bash
# Sets up what the browser-driven scripts need. Source this file, then call
# `browser_python` to get the path of a Python that can run Playwright.
#
# Uses CHROMIUM=/path/to/chromium when set; otherwise Playwright downloads one.

browser_python() {
  local root venv
  root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  venv="$root/.venv/browser"
  python3 -m venv "$venv"
  "$venv/bin/pip" install --quiet -r "$root/scripts/requirements-browser.txt"
  if [[ -z "${CHROMIUM:-}" ]]; then
    "$venv/bin/playwright" install chromium >&2
    # On GitHub's runners the system libraries Chromium needs may be missing.
    if [[ -n "${CI:-}" ]]; then sudo "$venv/bin/playwright" install-deps chromium >&2; fi
  fi
  echo "$venv/bin/python"
}
