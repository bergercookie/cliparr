# AGENTS.md

Rules for any AI agent or human working in this repo. Read fully before editing.

## What this is

Cliparr: self-hosted server to save and replay favorite moments (clips, quotes)
from a Jellyfin library. Users log in with their Jellyfin credentials.

- `server/` Go backend (stdlib `net/http`, SQLite). Serves the API and the web UI.
- `app/` Flutter frontend (web, Android, Linux).
- `docs/` Markdown docs, built with Sphinx + MyST.
- `docker/`, `packaging/`, `scripts/`, `.github/workflows/` build and release.

## Golden rule: use `just`

Everything goes through the `justfile`. Do not invent your own commands.

- `just` lists all recipes.
- `just check` runs lint + unit tests. Run it before every commit.
- `just ci` runs everything CI runs.
- CI and pre-commit only call `just`. Keep logic in the justfile or `scripts/`, never in workflow YAML.

## Code style

- Keep it simple, readable and boring. Small functions, clear names.
- Match the style of the surrounding code. Do not add dependencies without a reason.
- Go: standard library first. Wrap errors with `%w`. No globals. Add a test with every behavior change.
- Dart: run `just app-lint`. Keep widgets small. No state-management package.
- Bash: `set -euo pipefail`. Must pass shellcheck.
- Docs live in `docs/*.md` (Markdown only). Update them when behavior changes.
  Write for a reader who has never seen the code or this repo's history: say what to do first,
  keep one task per page, and explain jargon. If a page only makes sense with context from a
  change, rewrite it. Verify every command and config snippet you document (`docker compose config`,
  `caddy validate`).

## Security rules

- Never log or return passwords or tokens.
- Never forward Cliparr credentials to Jellyfin. Always use the user's own Jellyfin token.
- Clips belong to one Jellyfin user. Every query must filter by the user id.

## Git rules (mandatory)

- Commit as `Nikos Koukis <nickkouk@gmail.com>` (already set in local git config; do not change it).
- Do NOT add `Co-Authored-By`, `Claude-Session`, "Generated with Claude Code" or any other
  AI/tool mention or link to commit messages, PR descriptions or code comments.
- Commit messages: short imperative subject (max 72 chars), optional body explaining why.
- Do not push, tag or open PRs unless asked.

## Android releases

- The APK must always be signed with the project key (GitHub secrets `ANDROID_KEYSTORE_*`),
  otherwise Android refuses in-place updates. Never publish a debug-signed APK. See `docs/releasing.md`.
- The app learns its own version from `--dart-define=CLIPARR_VERSION` (set by the `just` recipes).

## Versions and releases

- Release: push a git tag `vX.Y.Z`. Nightly: next minor + `.postN` suffix (see `docs/releasing.md`).
- Never edit generated files in `server/internal/web/dist/`.
