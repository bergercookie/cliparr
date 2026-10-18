# Releasing

## Stable release

Push a tag of the form `vMAJOR.MINOR.PATCH`:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The release workflow builds everything and creates the GitHub release with:

- `cliparr-0.2.0.apk` (Android)
- `cliparr_0.2.0_amd64.deb` and `Cliparr-0.2.0-x86_64.AppImage` (Linux)
- `SHA256SUMS`, which the Android app uses to verify downloads
- Docker images `ghcr.io/bergercookie/cliparr` and `ghcr.io/bergercookie/cliparr-debug` for amd64 and arm64, tagged `0.2.0`, `0.2` and `latest`

Nothing is published until everything has been built: the Docker images are pushed only after the APK, the Linux packages and the extension succeeded, so a failed release leaves nothing half-published.

Before anything is published, the workflow installs the previous release's APK on an emulator and updates it to the new one. If Android would refuse that update (wrong signing key, version not higher) or the new version does not start, the release stops. See [Development](development.md#android-upgrade-test).

## Nightly releases

Every night at 02:17 UTC the nightly workflow publishes a prerelease, unless nothing changed since the previous nightly. You can also start it by hand from the Actions tab, which always builds.

The version takes the latest stable version, raises the **minor** number, keeps the patch number and adds `.postN.YYYYMMDD`: a counter of nightlies for that version, then the UTC date of the build.

| Latest stable tag | Nightlies |
| --- | --- |
| `v1.2.3` | `1.3.3.post1.20261017`, `1.3.3.post2.20261018`, ... |
| `v1.3.0` (cut later) | `1.4.0.post1.20261101`, ... |
| none yet | `0.1.0.post1.20261017`, ... |

Two nightlies on the same day get different counters (`post1`, `post2`) and the same date.

Nightly images are tagged with the full version and `nightly`. The 14 newest nightly releases are kept; older ones and their tags are deleted automatically. `scripts/version.sh` implements these rules and `scripts/test-version.sh` tests them.

## One-time setup

### Sign the Android app

Android installs an update only if it is signed with the same key as the installed app. Create the key once:

```sh
just android-keystore
```

The script writes the key and four small files into `build/signing/`, and prints the `gh secret set` commands that store them as repository secrets:

| Secret | Contents |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | The keystore file |
| `ANDROID_KEYSTORE_PASSWORD` | Its password |
| `ANDROID_KEY_PASSWORD` | The key password (same value) |
| `ANDROID_KEY_ALIAS` | `cliparr` |

**Back up `build/signing/` somewhere safe.** If the key is lost, nobody can update the app and everyone must uninstall it first. The release workflow refuses to publish an APK until the secrets exist, so an unsigned or wrongly signed APK can never reach users.

### Make the Docker images public

GitHub creates container packages as private. After the first release, open the package under your profile's Packages, then Package settings, and set the visibility to public so people can `docker pull` without logging in.

### Documentation site and coverage

See [Development](development.md#coverage-reports) for the Pages and Codecov setup.
