# Cliparr: the one entry point for building, checking and releasing.
# Run `just` to list the recipes. Install the tools with `devbox shell`.
# CI and the git hooks only call recipes from this file.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Version baked into builds. Releases pass the real one, e.g. `VERSION=1.2.3 just build`.
version := env("VERSION", "dev")
# GitHub repository, used for image names and releases.
repo := env("GITHUB_REPOSITORY", "bergercookie/cliparr")
flutter_version := `cat .flutter-version`

[private]
default:
    @just --list --unsorted

# --- development -------------------------------------------------------------

# Install the git hook that lints before every commit.
[group('dev')]
setup:
    pre-commit install

# Run the server against a fake Jellyfin (users alice and bob, password "pw") on http://localhost:8080.
[group('dev')]
dev:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    cd server
    go run ./cmd/fakejellyfin &
    CLIPARR_JELLYFIN_URL=http://127.0.0.1:8096 CLIPARR_DATA_DIR=../build/dev-data go run ./cmd/cliparr

# --- build ---------------------------------------------------------------------

# Build the web frontend into the server's embedded directory.
[group('build')]
build-web:
    cd app && flutter build web --release --dart-define=CLIPARR_VERSION={{ version }}
    find server/internal/web/dist -mindepth 1 ! -name .gitkeep -delete
    cp -r app/build/web/. server/internal/web/dist/

# Build the server binary (with the web frontend if build-web ran first) into build/cliparr.
[group('build')]
build-server:
    cd server && CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version={{ version }}" -o ../build/cliparr ./cmd/cliparr

# Build frontend and server.
[group('build')]
build: build-web build-server

# Build the server and debug Docker images locally (cliparr:dev and cliparr-debug:dev).
[group('build')]
docker-build:
    docker build -f docker/Dockerfile -t cliparr:dev \
        --build-arg VERSION={{ version }} --build-arg FLUTTER_VERSION={{ flutter_version }} .
    docker build -f docker/Dockerfile.debug -t cliparr-debug:dev .

# --- checks --------------------------------------------------------------------

# Lint everything through pre-commit (the same hooks run on every commit).
[group('check')]
lint:
    pre-commit run --all-files

# gofmt, go vet and golangci-lint.
[group('check')]
lint-go:
    #!/usr/bin/env bash
    cd server
    unformatted="$(gofmt -l .)"
    if [[ -n "$unformatted" ]]; then echo "Not gofmt-ed:"; echo "$unformatted"; exit 1; fi
    go vet ./...
    golangci-lint run

# dart format and flutter analyze.
[group('check')]
lint-app:
    cd app && dart format --set-exit-if-changed lib test && flutter analyze

# shellcheck and shfmt on every bash script.
[group('check')]
lint-bash:
    #!/usr/bin/env bash
    mapfile -t scripts < <(find . -name '*.sh' -not -path './app/*' -not -path './.git/*' -not -path './build/*')
    shellcheck "${scripts[@]}"
    shfmt -d -i 2 -ci "${scripts[@]}"

# hadolint on the Dockerfiles.
[group('check')]
lint-docker:
    hadolint docker/Dockerfile docker/Dockerfile.debug

# actionlint on the GitHub workflows.
[group('check')]
lint-actions:
    actionlint

# Known-vulnerability scan of the Go dependencies and standard library.
[group('check')]
vuln:
    cd server && govulncheck ./...

# Go unit and API tests. Writes build/coverage/server.out.
[group('check')]
test-go:
    #!/usr/bin/env bash
    mkdir -p build/coverage
    (cd server && go test -race -covermode=atomic -coverprofile=../build/coverage/raw.out ./...)
    # Leave out the fake servers and the generated frontend bundle: they are test tooling.
    grep -v -e '/fakejellyfin/' -e '/cmd/fakeauthelia/' -e '/internal/web/' build/coverage/raw.out > build/coverage/server.out
    rm build/coverage/raw.out

# Flutter unit and widget tests. Writes build/coverage/app.lcov.
[group('check')]
test-app:
    mkdir -p build/coverage
    cd app && flutter test --coverage
    # Make the file paths relative to the repository root, as the coverage service expects.
    sed 's#^SF:lib/#SF:app/lib/#' app/coverage/lcov.info > build/coverage/app.lcov

# Tests for the release version script.
[group('check')]
test-scripts:
    scripts/test-version.sh

# All unit tests.
[group('check')]
test: test-go test-app test-scripts

# End-to-end test: real server behind Caddy and a fake Authelia (needs caddy, curl, jq).
[group('check')]
e2e:
    e2e/run.sh

# Print the coverage of the last `just test` (also added to the GitHub job summary in CI).
[group('check')]
coverage:
    scripts/coverage-summary.sh

# Lint and unit tests: run this before every commit.
[group('check')]
check: lint test

# Everything CI runs.
[group('check')]
ci: check vuln e2e docs

# --- documentation -------------------------------------------------------------

# Build the documentation into docs/_build/html (warnings are errors).
[group('docs')]
docs:
    python3 -m venv .venv/docs
    .venv/docs/bin/pip install --quiet -r docs/requirements.txt
    .venv/docs/bin/sphinx-build -W --keep-going -b html docs docs/_build/html

# Build the documentation and serve it on http://localhost:8000.
[group('docs')]
docs-serve: docs
    python3 -m http.server --directory docs/_build/html 8000

# --- release (used by the GitHub workflows) -------------------------------------

# Exit successfully only if there are new commits since the last nightly.
[group('release')]
nightly-needed:
    scripts/version.sh has-changes

# Print the version of the next nightly build.
[group('release')]
nightly-version:
    scripts/version.sh next-nightly

# Build the .deb and AppImage into dist/. Usage: just package-linux 1.2.3
[group('release')]
package-linux release:
    #!/usr/bin/env bash
    release="{{ release }}"
    (cd app && flutter build linux --release --dart-define=CLIPARR_VERSION="$release" \
      --build-name "${release%%.post*}" --build-number "$(git rev-list --count HEAD)")
    bundle=app/build/linux/x64/release/bundle
    packaging/build-deb.sh "$release" "$bundle" dist
    packaging/build-appimage.sh "$release" "$bundle" dist

# Create the Android release signing key and print how to store it as GitHub secrets (once per project).
[group('release')]
android-keystore:
    scripts/android-keystore.sh

# Build the Android APK into dist/. Usage: just package-apk 1.2.3
[group('release')]
package-apk release:
    #!/usr/bin/env bash
    release="{{ release }}"
    (cd app && flutter build apk --release --dart-define=CLIPARR_VERSION="$release" \
      --build-name "${release%%.post*}" --build-number "$(git rev-list --count HEAD)")
    mkdir -p dist
    cp app/build/app/outputs/flutter-apk/app-release.apk "dist/cliparr-$release.apk"

# Build and push the multi-arch server and debug images to ghcr.io. Usage: just docker-publish 1.2.3
[group('release')]
docker-publish release:
    #!/usr/bin/env bash
    release="{{ release }}"
    repo="{{ repo }}"
    image="ghcr.io/${repo,,}"
    if [[ "$release" == *.post* ]]; then
      tags=("$release" nightly)
    else
      tags=("$release" "${release%.*}" latest)
    fi
    publish() { # publish DOCKERFILE IMAGE
      local args=()
      for tag in "${tags[@]}"; do args+=(-t "$2:$tag"); done
      docker buildx build --platform linux/amd64,linux/arm64 --push -f "$1" "${args[@]}" \
        --build-arg VERSION="$release" --build-arg FLUTTER_VERSION={{ flutter_version }} .
    }
    publish docker/Dockerfile "$image"
    publish docker/Dockerfile.debug "$image-debug"

# Create the GitHub release from dist/. Usage: just github-release 1.2.3 [prerelease]
[group('release')]
github-release release kind="stable":
    #!/usr/bin/env bash
    cd dist
    sha256sum -- * > SHA256SUMS
    flags=()
    [[ "{{ kind }}" == "prerelease" ]] && flags+=(--prerelease)
    # Nightlies create their tag here, so pin it to the commit that was built.
    [[ -n "${GITHUB_SHA:-}" ]] && flags+=(--target "$GITHUB_SHA")
    gh release create "v{{ release }}" -- * \
      --repo "{{ repo }}" --title "Cliparr {{ release }}" --generate-notes "${flags[@]}"

# Delete all but the newest nightly releases and their tags. Usage: just nightly-prune [keep]
[group('release')]
nightly-prune keep="14":
    #!/usr/bin/env bash
    gh release list --repo "{{ repo }}" --limit 200 --json tagName,publishedAt \
      --jq 'map(select(.tagName | test("\\.post[0-9]+$"))) | sort_by(.publishedAt) | reverse | .[{{ keep }}:][].tagName' |
      while read -r tag; do
        gh release delete "$tag" --repo "{{ repo }}" --cleanup-tag --yes
      done
