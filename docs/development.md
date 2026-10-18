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

Devbox's Flutter can be a patch release behind the version in `.flutter-version`, which CI, the Docker image and the releases use. That is fine for everyday work, but `flutter pub get` may then rewrite `app/pubspec.lock`: do not commit that change unless you updated dependencies on purpose.

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

## Logo

The logo is `packaging/cliparr.svg`: scissors cutting a film strip. It is the only source. The Linux packages use it as it is. The PNG icons of the web app, the Android launcher and the browser extension are rendered from it, because their manifests do not accept SVG. After changing the logo, run `just icons` (it uses Playwright like `just screenshots`, so `CHROMIUM=/path/to/chromium` works here too) and commit the PNGs it writes.

## Browser extension

The code lives in `extension/` and is loaded as it is; `just package-extension <version>` makes the Chrome and Firefox zips (they differ only in the generated manifest). To try it by hand in Chrome, run `scripts/build-extension.sh`, then load `build/extension/chrome` as an unpacked extension. `SCREENSHOT_DIR=docs/screenshots just e2e-extension` refreshes the picture used in the docs.

Three checks cover the extension:

- `just lint-extension` runs ESLint on the scripts (undefined or unused names, `==`, `eval` and similar) and then Firefox's `web-ext` linter on the generated manifest. The settings are in `extension/eslint.config.cjs`. Nothing needs installing: `npx` fetches a pinned ESLint, so the extension itself keeps zero npm dependencies.
- `just test-extension` runs the unit tests, including ones that fail if the manifest asks for more access than the extension promises (never `<all_urls>`, only the permissions it uses today).
- `just e2e-extension` loads the built extension into a real Chromium and bookmarks a clip in a stand-in for Jellyfin's player. It runs on Chrome only: Playwright cannot load extensions into Firefox, so Firefox relies on the shared code, the unit tests and the `web-ext` lint.
- `just e2e-jellyfin` does the same on Jellyfin's own web player; see the next section.

Chromium and Firefox parse some input differently (for example a space in a host name), so keep decisions like "is this a valid address" in `lib.js` with explicit rules and unit tests, not in the URL parser alone.

## Tests against a real Jellyfin

The fake Jellyfin in the other tests is quick, but it only knows what we taught it. `just e2e-jellyfin [VERSION]` checks Cliparr against the real thing. It downloads a Jellyfin release from `repo.jellyfin.org` (kept in `build/jellyfin/`), starts it with an empty database and a generated ten-minute movie, starts Cliparr, and runs three groups of checks from `e2e/jellyfin/`:

- `server_checks.py`: login, search, streaming, preview pictures, who may see which clip, and the resume point.
- `web_checks.py`: Jellyfin's web client in a browser. The "Open in Jellyfin web" link must lead to the movie and Play must continue at the clip start, and "Play on a device" must start the clip on the open web client.
- `extension_checks.py`: the browser extension on Jellyfin's real player.

`VERSION` is a release such as `10.11.11` or `12.2`, or `latest-stable` (the default). Only the releases `repo.jellyfin.org` still offers can be tested; today that is 10.11 and 12.x, and CI runs the oldest of them and the newest. It needs `ffmpeg`, `jq` and `zip` besides the usual tools, and takes a few minutes: the test browser cannot play H.264, so Jellyfin converts the video first, which is slow on a small machine. Set `JELLYFIN_E2E_KEEP=1` to keep the temporary folder with Jellyfin's data and log when something fails.

The movie has no online metadata on purpose: the checks must not depend on what a metadata service thinks a file is.

## Android upgrade test

An Android update fails if the new APK has a different signing key, a different package name or a lower version code, and nobody notices until a user complains. Two scripts guard against that:

- **Static check** (`just upgrade-static OLD NEW`): needs only the Android SDK. It compares the two APKs and fails on any of those three mistakes. It runs in a few seconds.
- **Emulator test** (`just upgrade-check OLD NEW`): needs an emulator or phone that `adb` can see. It installs `OLD`, starts it, updates to `NEW` the way Android's updater does, and checks the new version starts. It also checks that Android rejects an update signed with a different key.

`just upgrade-test` builds two throwaway versions of the current code (`0.0.1` and `0.0.2`, signed with a temporary key that is not the release key) and runs both checks. CI does this on every pull request. The release workflow does it again before publishing, using the APK of the previous release as `OLD`, so a release that existing users could not install never gets published.

## Code quality checks

Besides lint and tests (`just check`, run by `ci.yml`), the workflow `.github/workflows/code-quality.yml` runs on every push to `master`, every pull request and once a week:

- **CodeQL** reads the Go server, the extension and script JavaScript, and the workflow files themselves for security problems and common bugs. It uses the `security-and-quality` query set. Findings appear under *Security*, *Code scanning* on GitHub, and on the pull request that causes them. It reports; it does not block a merge unless you add a branch protection rule for it.
- **Dependency review** (pull requests only) fails the check when a pull request adds a Go module, Dart package or action that has a known high-severity vulnerability. `govulncheck` in `ci.yml` covers the vulnerabilities already in the code.

CodeQL needs GitHub's runners, so there is no local `just` recipe for it. The closest local check is `just lint-go`, whose golangci-lint includes the gosec security rules.

## Coverage reports

CI uploads coverage to [Codecov](https://about.codecov.io/), which comments on pull requests and serves the README badge. One-time setup: sign in to Codecov with GitHub, enable this repository, and add the upload token as the repository secret `CODECOV_TOKEN`. Until then the upload is skipped and CI still passes. Coverage also appears in each CI run's summary.

## Documentation

The docs are Markdown in `docs/`, readable on GitHub and built into a site with Sphinx and MyST (`just docs`). Every change to the docs is built in CI. Pushes to `master` also publish the site to GitHub Pages once you have enabled it: under Settings, Pages, set the Source to GitHub Actions, then add the repository variable `PAGES_ENABLED` with the value `true` (Settings, Secrets and variables, Actions, Variables). Until then the workflow only builds the docs and says so in its log.

Write for someone who has never seen the code: say what to do before explaining why, and keep each page about one task.
