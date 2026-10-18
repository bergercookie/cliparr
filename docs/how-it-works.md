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

The [browser extension](extension.md) skips the password step: it runs inside Jellyfin's web page, which already holds the user's Jellyfin token, and sends it as `Authorization: Jellyfin <token>`. Cliparr asks Jellyfin (`/Users/Me`) whose token it is, caches the answer for five minutes, and then proceeds exactly as after a password login. Nothing is stored, and API keys, which belong to no user, are refused.

With [single sign-on](sso.md) the proxy vouches for the user instead, and Cliparr uses an API key. Everything else stays the same.

## Preview pictures

When a card first needs its picture, Cliparr starts `ffmpeg`, which reads the movie directly from Jellyfin (with your own token) and jumps to the middle of the clip using HTTP range requests, so only a small part of the file is transferred. The frame is saved as a small JPEG in the `thumbnails` folder of the data directory and served from there afterwards. Changing a clip's start or end, or deleting it, removes its picture. If ffmpeg is missing or fails, the card shows the movie's poster from Jellyfin instead.

## Opening a clip in Jellyfin

Jellyfin's web client has no address format with a start time, so Cliparr uses Jellyfin's API:

- **Play on a device:** Cliparr asks Jellyfin for the running clients that you may remote-control, and sends the chosen one a "play this item at this position" command. Cliparr only accepts devices from that list, so one user cannot start playback on someone else's device.
- **Open in Jellyfin web:** Cliparr stores the bookmark's start as your resume position for the item and gives the app the Jellyfin web address (built from `CLIPARR_JELLYFIN_PUBLIC_URL`). Jellyfin's Resume button then starts there.

## Playing a clip

The player asks Cliparr for `/api/items/<id>/stream`. Cliparr forwards the request to Jellyfin's original-file endpoint and relays the answer, including HTTP range requests, so seeking works. The Jellyfin address and token never reach the browser. Playback stops at the clip's end and jumps back to its start.

### Audio a browser cannot decode

Browsers cannot decode some audio formats, among them Dolby Digital (AC3, E-AC3), DTS and TrueHD, which are common on movies from discs. Played as it is, such a video shows its picture in silence.

When Jellyfin says an item has such audio, the player keeps the picture on the original file, muted, and plays the sound from `/api/items/<id>/audio?start=<milliseconds>` next to it. Cliparr asks Jellyfin to convert the sound to stereo MP3 starting at that time, and relays the answer. Because Jellyfin converts the sound as it is being read, it cannot be seeked: after every jump of the picture, and whenever the two drift apart, the player asks for a new copy that starts where the picture is. So after a jump the sound is silent for a moment. Jellyfin does this conversion with its own ffmpeg, so the Jellyfin user needs to be allowed to transcode audio (the default).

If the sound cannot be had either, the player shows a muted speaker next to the time.

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
| `GET /api/items/{id}` | One item; `audioPlayable` tells whether browsers can decode its audio |
| `GET /api/items/{id}/stream` and `/image` | Video and poster, relayed from Jellyfin |
| `GET /api/items/{id}/audio?start=` | The sound as stereo MP3 from `start` milliseconds on, for items whose own audio browsers cannot decode |
| `GET/POST /api/clips`, `GET/PUT/DELETE /api/clips/{id}` | Manage clips; `GET /api/clips` accepts `q` and `tag` |
| `GET /api/clips/{id}/thumbnail` | Preview picture of the clip (JPEG, or the poster if it cannot be made) |
| `GET /api/clips/{id}/jellyfin` | How to open the clip in Jellyfin: web address, start, current resume point, controllable devices |
| `POST /api/clips/{id}/jellyfin/play` | Play the clip on a Jellyfin device: `{"deviceId": "..."}` |
| `POST /api/clips/{id}/jellyfin/resume` | Set the Jellyfin resume point to the clip start |

Send the token as `Authorization: Bearer <token>`, or a Jellyfin token as `Authorization: Jellyfin <token>`.
