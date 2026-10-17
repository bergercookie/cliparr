#!/usr/bin/env bash
# Prints the test coverage written by `just test-go` and `just test-app`.
# In GitHub Actions the table also lands in the job summary.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
server=build/coverage/server.out
app=build/coverage/app.lcov

for file in "$server" "$app"; do
  [[ -f "$file" ]] || {
    echo "$file is missing. Run 'just test' first." >&2
    exit 1
  }
done

server_pct="$(cd server && go tool cover -func=../"$server" | awk '/^total:/ {print $3}')"
# lcov: LF = lines found, LH = lines hit.
app_pct="$(awk -F: '/^LF:/ {f += $2} /^LH:/ {h += $2} END {if (f) printf "%.1f%%", 100 * h / f; else print "n/a"}' "$app")"

table="$(
  printf '| Part | Line coverage |\n| --- | --- |\n| Server (Go) | %s |\n| App (Flutter) | %s |\n' "$server_pct" "$app_pct"
)"
echo "$table"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  printf '### Test coverage\n\n%s\n' "$table" >>"$GITHUB_STEP_SUMMARY"
fi
