# Cliparr app

Flutter app for web, Android and Linux. The web build is bundled into the server; the Android and Linux builds are published with every release.

Use the `just` recipes from the repository root (`just build-web`, `just test-app`, `just lint-app`, `just package-apk <version>`). See [Development](../docs/development.md).

Code layout under `lib/`: `api/` (server client and models), `state/` (login session, settings), `update/` (version check and APK install), `screens/` and `widgets/` (UI).
