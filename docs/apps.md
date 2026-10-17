# Apps

There is also a [browser extension](extension.md) that bookmarks moments straight from Jellyfin's web player.

The web interface needs no installation: open your server's address in a browser. The apps are for phones and desktops where you prefer a native player or need formats the browser cannot play.

All builds are attached to each [GitHub release](https://github.com/bergercookie/cliparr/releases).

## Android

1. Download `cliparr-<version>.apk` from the latest release on your phone.
2. Open it. Android asks whether the browser (or file manager) may install apps; allow it.
3. Start Cliparr and enter the address of your server, for example `http://192.168.1.10:8080` or `https://cliparr.example.com`. Plain `http://` addresses work, since most home servers have no certificate.
4. Log in with your Jellyfin account.

If your server is behind single sign-on, use the separate app host name described in [Single sign-on](sso.md#the-apps-and-the-browser-extension-cannot-do-sso).

## Linux

**Debian and Ubuntu** (`.deb`):

```sh
sudo apt install ./cliparr_<version>_amd64.deb
```

This also installs the libraries Cliparr needs (GTK and mpv). Start it from your application menu or with `cliparr`.

**Any distribution** (AppImage):

```sh
chmod +x Cliparr-<version>-x86_64.AppImage
./Cliparr-<version>-x86_64.AppImage
```

If it complains about FUSE, install `libfuse2`, or run it with `--appimage-extract-and-run`.

## Settings

Open the gear icon in the top bar.

- **Server address**: change which server the app talks to. You are logged out and asked to log in again.
- **Server version** and **App version**: handy when reporting a problem.
- **App updates** (apps only, not the web interface): choose a channel and see whether you are up to date.
  - **Stable** follows tagged releases.
  - **Nightly** follows every build, including the automatic nightly ones. Nightlies are newer but less tested.

The app looks at the GitHub releases of the project, so it needs internet access to check. A development build (one you compiled yourself) reports "Development build" instead of comparing versions.

### Updating the Android app

When a newer version exists in your channel, tap **Download and install**. Cliparr downloads the APK from GitHub, checks it against the release's SHA-256 checksum, and hands it to Android's installer, which asks you to confirm. The first time, Android also asks you to allow Cliparr to install apps.

Android only accepts an update that is signed with the same key as the installed app, which is the case for every official release. If you get "App not installed", the installed copy came from somewhere else (for example your own debug build). Uninstall it once and install the release.

On Linux the settings page tells you when an update exists; install the new `.deb` or AppImage yourself.
