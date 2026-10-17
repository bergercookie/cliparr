# Development

## Set up

Install [Devbox](https://www.jetify.com/devbox) and Docker, then:

```sh
git clone https://github.com/bergercookie/cliparr
cd cliparr
devbox shell     # installs go, flutter, just, linters, caddy, ... (not Docker)
just setup       # git hook that lints before every commit
```

Everything you do in this repository goes through [`just`](https://just.systems/). Run `just` to list the recipes. CI runs the same recipes, so a green `just ci` locally means a green pipeline.

Building the Android app additionally needs a JDK 17 and the Android SDK; Devbox does not install them.

## Layout

| Path | Contents |
| --- | --- |
| `server/` | Go backend. `internal/api` handlers, `internal/store` SQLite, `internal/jellyfin` client, `internal/fakejellyfin` test double |
| `app/` | Flutter app for web, Android and Linux |
| `extension/` | Browser extension (plain JavaScript, no build tools): `lib.js` logic, `content.js` button, `background.js`, `options.*` |
| `e2e/` | End-to-end test: real server behind Caddy and a stand-in for Authelia |
| `docker/` | Server image and the troubleshooting sidecar |
| `packaging/` | Scripts that build the `.deb` and AppImage |
| `scripts/` | Release versioning, Android key generation, coverage summary |
| `docs/` | These pages |

## Everyday commands

| Command | What it does |
| --- | --- |
| `just dev` | Runs the server on <http://localhost:8080> against a fake Jellyfin (users `alice` and `bob`, password `pw`) |
| `just build-web` then `just build-server` | Builds the web interface into the server binary, then the binary into `build/cliparr` |
| `just check` | Lint and unit tests; run before each commit |
| `just e2e` | End-to-end tests (needs `caddy`, `curl`, `jq`) |
| `just ci` | Everything CI runs |
| `just docs-serve` | Builds these docs and serves them on <http://localhost:8000> |
| `just docker-build` | Builds the Docker images locally |

To work on the interface with the fake Jellyfin: run `just dev`, build the web app once with `just build-web`, and restart. The web build must be served by the Cliparr server itself, because login uses a same-origin cookie.

## Tests

- Go tests live next to the code. API tests run the real handlers against `fakejellyfin`, so they need no network. Tests of the preview pictures run the real `ffmpeg` and skip themselves when it is not installed (Devbox and CI have it).
- Flutter tests cover the API client, version logic, the update checker and the screens.
- `just test-extension` runs the extension's logic tests with Node. `just e2e-extension` loads the extension into a real Chromium, plays a video on a stand-in Jellyfin web player and checks what reaches Cliparr. `just lint-extension` validates the Firefox build with `web-ext`.
- `e2e/run.sh` starts the real binary and checks password login, SSO through Caddy, and header spoofing. The Authelia in that test is a small stand-in (`server/cmd/fakeauthelia`) that follows Authelia's forward-auth contract; it is not Authelia itself.
- `just coverage` prints coverage after `just test`.

Add a test with every behaviour change. See `AGENTS.md` for the conventions.

## Screenshots

`just screenshots` rebuilds the web app, starts the server against the fake Jellyfin (which streams a generated one-minute test video, so it needs `ffmpeg`) with a few sample clips, and photographs the main screens in a headless Chromium into `docs/screenshots/` (light, dark and phone-sized). It uses Playwright, which downloads a Chromium on first run; set `CHROMIUM=/path/to/chromium` to use one you already have. Re-run it after UI changes and commit the new images. The sample quotes are invented on purpose, so the images do not attribute real lines to real films.

## Browser extension

The code lives in `extension/` and is loaded as it is; `just package-extension <version>` makes the Chrome and Firefox zips (they differ only in the generated manifest). To try it by hand in Chrome, run `scripts/build-extension.sh`, then load `build/extension/chrome` as an unpacked extension. `SCREENSHOT_DIR=docs/screenshots just e2e-extension` refreshes the picture used in the docs.

Chromium and Firefox parse some input differently (for example a space in a host name), so keep decisions like "is this a valid address" in `lib.js` with explicit rules and unit tests, not in the URL parser alone.

## Android upgrade test

An Android update fails if the new APK has a different signing key, a different package name or a lower version code, and nobody notices until a user complains. Two scripts guard against that:

- **Static check** (`just upgrade-static OLD NEW`): needs only the Android SDK. It compares the two APKs and fails on any of those three mistakes. It runs in a few seconds.
- **Emulator test** (`just upgrade-check OLD NEW`): needs an emulator or phone that `adb` can see. It installs `OLD`, starts it, updates to `NEW` the way Android's updater does, and checks the new version starts. It also checks that Android rejects an update signed with a different key.

`just upgrade-test` builds two throwaway versions of the current code (`0.0.1` and `0.0.2`, signed with a temporary key that is not the release key) and runs both checks. CI does this on every pull request. The release workflow does it again before publishing, using the APK of the previous release as `OLD`, so a release that existing users could not install never gets published.

## Coverage reports

CI uploads coverage to [Codecov](https://about.codecov.io/), which comments on pull requests and serves the README badge. One-time setup: sign in to Codecov with GitHub, enable this repository, and add the upload token as the repository secret `CODECOV_TOKEN`. Until then the upload is skipped and CI still passes. Coverage also appears in each CI run's summary.

## Documentation

The docs are Markdown in `docs/`, readable on GitHub and built into a site with Sphinx and MyST (`just docs`). Every change to the docs is built in CI. Pushes to `master` also publish the site to GitHub Pages once you have enabled it: under Settings, Pages, set the Source to GitHub Actions, then add the repository variable `PAGES_ENABLED` with the value `true` (Settings, Secrets and variables, Actions, Variables). Until then the workflow only builds the docs and says so in its log.

Write for someone who has never seen the code: say what to do before explaining why, and keep each page about one task.
