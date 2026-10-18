# Troubleshooting

## Look at the logs first

```sh
docker compose logs --tail 50 cliparr
```

Every request is logged with its status code. Messages starting with `level=WARN` or `level=ERROR` explain most problems.

## Common problems

| What you see | Likely cause and fix |
| --- | --- |
| The container keeps restarting; the log says `the data folder … is not writable` (or, in 0.1.0, `unable to open database file (14)`) | The user inside the container is not allowed to write to the data folder. Set `PUID` and `PGID` to your own user and group (`id -u`, `id -g`) and Cliparr takes care of the folder. If you set `user:` yourself, the folder must belong to that user. See [Data folder and permissions](configuration.md#data-folder-and-permissions). |
| The log says `PUID is 0, which is root` | Cliparr does not run as root. Use your own numbers from `id -u` and `id -g`, which are not 0. |
| The browser shows `SSL_ERROR_INTERNAL_ERROR_ALERT` or `ERR_SSL_PROTOCOL_ERROR` behind Caddy, and Cliparr's log shows nothing | Caddy has no certificate for that name. See [If the browser shows a TLS error](reverse-proxy.md#if-the-browser-shows-a-tls-error). |
| The extension says it cannot reach Cliparr; Firefox's console says `CORS request did not succeed` with status `(null)` | The request did not complete at all: wrong address or port, Cliparr stopped, or an `https` address whose certificate the browser does not trust. Open the address in a browser tab: accept the certificate warning once if there is one. See [the extension page](extension.md#if-the-extension-cannot-reach-cliparr). |
| The logs say `cannot reach Jellyfin yet`, or login says "could not reach Jellyfin" | `CLIPARR_JELLYFIN_URL` is wrong or not reachable from the container. `localhost` does not work inside a container; use the service name or the host's LAN address. Test with the [debug container](#debug-container). |
| Login says "invalid Jellyfin username or password" | Try the same credentials in Jellyfin itself. Users who sign in through an SSO plugin have no password; see [Single sign-on](sso.md). |
| You are logged out again right after logging in, behind a proxy | The proxy must forward `X-Forwarded-Proto` (Caddy does by default). Check that you open the HTTPS address, not plain HTTP. |
| The picture plays but there is no sound, and the player shows a muted speaker | The video has audio that browsers cannot decode (Dolby Digital, DTS) and Cliparr could not get Jellyfin to convert it. In Jellyfin, check that the user may transcode audio (user settings, Playback) and that Jellyfin's ffmpeg works. See [Audio a browser cannot decode](how-it-works.md#audio-a-browser-cannot-decode). |
| The picture plays but there is no sound, and no muted speaker shows | Check the volume of the tab and of the system first. If other videos play with sound, [report it](https://github.com/bergercookie/cliparr/issues) with the audio format of the file (`ffprobe -hide_banner file.mkv`). |
| A clip shows "Could not play this video" | Browsers play only some formats. Try the Android or Linux app, or convert the file to H.264 and AAC in an MP4. |
| Cards show a poster or a grey movie icon instead of a scene | The preview picture could not be made. Check that `ffmpeg` exists in the container (`docker compose exec cliparr ffmpeg -version`) and that the bookmark's time lies inside the video. The logs say `thumbnail failed` with the reason. |
| "Open in Jellyfin web" opens the wrong address, or the logs warn about `CLIPARR_JELLYFIN_PUBLIC_URL` | Set `CLIPARR_JELLYFIN_PUBLIC_URL` to the address you use in your browser to open Jellyfin, for example `https://jellyfin.example.com`. |
| "No Jellyfin app that Cliparr can control is running" | Open the Jellyfin app on your TV, phone or computer first, then use the refresh button in the sheet. Browser tabs with Jellyfin open count too. |
| Clips are listed but the poster or video fails | The Jellyfin item was deleted or moved, or your Jellyfin user lost access to its library. |
| SSO: "your SSO username does not match any Jellyfin user" | The username from Authelia must exist in Jellyfin (capitalisation is ignored). |
| SSO: the login page appears even though Authelia logged you in | Cliparr ignores the header. Look for `ignoring proxy auth header sent from an untrusted address` in the logs: the address shown must be inside `CLIPARR_PROXY_AUTH_TRUSTED`. |
| Android update says "App not installed" | The installed app was signed with a different key. Uninstall it once, then install the release APK. |
| Android update is blocked | Allow Cliparr to install apps in Android's settings (it offers a shortcut), then try again. |

## Debug container

Every release also publishes `ghcr.io/bergercookie/cliparr-debug`: a [netshoot](https://github.com/nicolaka/netshoot) container (curl, dig, tcpdump and more) plus a script that checks the connections Cliparr relies on.

Add it to your Compose file so it shares the network with Cliparr:

```yaml
  debug:
    image: ghcr.io/bergercookie/cliparr-debug:latest
    profiles: [debug] # only starts when you ask for it
```

Run the checks:

```sh
docker compose --profile debug run --rm debug cliparr-diagnose http://cliparr:8080 http://192.168.1.10:8096
```

It reports whether each name resolves, whether Cliparr answers on `/healthz`, and whether Jellyfin is reachable. For anything else, open a shell: `docker compose --profile debug run --rm debug bash`.

## Reporting a bug

Open an issue at <https://github.com/bergercookie/cliparr/issues> with the server and app versions (Settings page), what you did, and the relevant log lines. Remove tokens and API keys first.
