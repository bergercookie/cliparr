# How it works

Cliparr is one Go program that serves a JSON API and the web interface. It keeps its data in a SQLite file and talks to Jellyfin on behalf of the logged-in user.

```text
 browser / app  ──►  Cliparr  ──►  Jellyfin
                       │
                    SQLite file (clips, sessions)
```

## What is stored

For each clip: the Jellyfin item id, the title at the time you saved it, start and end in milliseconds, your quote, note and tags. Cliparr never copies video or images. Login sessions are stored too, as described below.

## Logging in

1. You send your Jellyfin username and password to Cliparr.
2. Cliparr passes them to Jellyfin (`/Users/AuthenticateByName`). Jellyfin answers with an access token for that user and that "device" (it appears as *Cliparr Server* in your Jellyfin devices list).
3. Cliparr stores the Jellyfin token and hands **you** a separate random Cliparr token. The web interface keeps it in a cookie that scripts cannot read; the apps keep it on the device. The password is forgotten immediately.
4. From now on every request to Jellyfin uses your own Jellyfin token, so Jellyfin decides what you may see. Cliparr needs no administrator rights and no API key.

Sessions last 30 days. Only a hash of your Cliparr token is stored, so a stolen database cannot be used to log in as someone. The Jellyfin token must be stored in clear text, because Cliparr has to present it to Jellyfin: protect the data volume like you protect Jellyfin's own data.

With [single sign-on](sso.md) the proxy vouches for the user instead, and Cliparr uses an API key. Everything else stays the same.

## Playing a clip

The player asks Cliparr for `/api/items/<id>/stream`. Cliparr forwards the request to Jellyfin's original-file endpoint and relays the answer, including HTTP range requests, so seeking works. The Jellyfin address and token never reach the browser. Playback stops at the clip's end and jumps back to its start.

Jellyfin counts time in *ticks* of 100 nanoseconds (10,000,000 per second). Cliparr converts to milliseconds when it reads the runtime of an item, so clips and the API use milliseconds everywhere.

## API

All endpoints return JSON and need a login, except the first three.

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Liveness probe |
| `GET /api/info` | Name and version of the server (used by the apps to check an address) |
| `POST /api/auth/login` | Log in with `{"username", "password"}`; returns `{"token", "user"}` |
| `POST /api/auth/logout` | End the session |
| `GET /api/me` | The current user |
| `GET /api/items?q=` | Search movies and episodes |
| `GET /api/items/{id}/stream` and `/image` | Video and poster, relayed from Jellyfin |
| `GET/POST /api/clips`, `GET/PUT/DELETE /api/clips/{id}` | Manage clips; `GET /api/clips` accepts `q` and `tag` |

Send the token as `Authorization: Bearer <token>`.
