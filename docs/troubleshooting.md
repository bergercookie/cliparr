# Troubleshooting

## Look at the logs first

```sh
docker compose logs --tail 50 cliparr
```

Every request is logged with its status code. Messages starting with `level=WARN` or `level=ERROR` explain most problems.

## Common problems

| What you see | Likely cause and fix |
| --- | --- |
| Login says "could not reach Jellyfin" | `CLIPARR_JELLYFIN_URL` is wrong or not reachable from the container. `localhost` does not work inside a container; use the service name or the host's LAN address. Test with the [debug container](#debug-container). |
| Login says "invalid Jellyfin username or password" | Try the same credentials in Jellyfin itself. Users who sign in through an SSO plugin have no password; see [Single sign-on](sso.md). |
| You are logged out again right after logging in, behind a proxy | The proxy must forward `X-Forwarded-Proto` (Caddy does by default). Check that you open the HTTPS address, not plain HTTP. |
| A clip shows "Could not play this video" | Browsers play only some formats. Try the Android or Linux app, or convert the file to H.264 and AAC in an MP4. |
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
