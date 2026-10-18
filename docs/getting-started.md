# Getting started

## What you need

- A running Jellyfin server that the Cliparr container can reach over the network. Cliparr targets Jellyfin 10.9 and newer.
- Docker with Compose.
- A Jellyfin account. Cliparr uses it to log you in; no extra accounts are created.

## Install

Download the commented example and save it as `compose.yaml` on your server:

```sh
curl -fsSL -o compose.yaml https://raw.githubusercontent.com/bergercookie/cliparr/master/compose.example.yaml
```

Or create `compose.yaml` yourself with the same minimum:

```yaml
services:
  cliparr:
    image: ghcr.io/bergercookie/cliparr:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      CLIPARR_JELLYFIN_URL: http://192.168.1.10:8096
    volumes:
      - cliparr-data:/data

volumes:
  cliparr-data:
```

Set `CLIPARR_JELLYFIN_URL` to the address of Jellyfin **as seen from the container**:

- Jellyfin in the same Compose file: use its service name, e.g. `http://jellyfin:8096`.
- Jellyfin in another Compose project or on another machine: use the host's LAN address, e.g. `http://192.168.1.10:8096`. `localhost` will not work, because inside the container that means the container itself.

Start it with `docker compose up -d`, then open <http://localhost:8080>.

The volume `cliparr-data` holds one file, `cliparr.db`, with all clips and login sessions. Back it up like any other database.

For HTTPS and a proper domain name, continue with [Reverse proxy and HTTPS](reverse-proxy.md).

## Save your first clip

1. Log in with your Jellyfin username and password.

   ![The login page](screenshots/login.png)

2. Click **New clip** and search for a movie or episode. Click a result.

   ![Searching the library for a movie](screenshots/new-clip-search.png)
3. The video starts loading. Play or drag the slider to the moment you like, then click the flag next to **Start**. Move to the end and click the flag next to **End**. You can also type times such as `1:23` or `1:02:03`.
4. Click **Preview clip** to loop the range. Adjust until it feels right.
5. Add the quote, a note about why it matters to you, and some comma-separated tags. Click **Save clip**.

## Your bookmarks

The clips page shows every bookmark as a card: a picture from the middle of the scene, the title, your quote and tags. Use the search box for words in the quote, note or title.

![The clips page](screenshots/clips.png)

Each card has two buttons:

- **Play clip** plays the scene here, on a loop between your start and end.

  ![Playing a clip](screenshots/clip-player.png)

- **Open in Jellyfin** continues in Jellyfin itself, at the moment you bookmarked. See below.

The interface follows your device's light or dark setting and also works on phones.

### Open in Jellyfin at the exact moment

Jellyfin's web addresses cannot contain a start time, so Cliparr offers two ways to land on the right second:

![The Open in Jellyfin sheet](screenshots/open-in-jellyfin.png)

- **Play on a Jellyfin device.** Cliparr lists the Jellyfin apps that are running for you right now (your TV, phone, a browser tab) and tells the one you pick to play the movie from the bookmark's start. This needs Jellyfin to be open on that device.
- **Open in Jellyfin web.** Cliparr sets your *resume point* for that movie to the bookmark's start and opens it in Jellyfin; press **Resume** there. Because this replaces your real resume position, Cliparr asks first and tells you what it will replace.

For the web option to open the right address, set `CLIPARR_JELLYFIN_PUBLIC_URL` to the address you use in your browser (see [Configuration](configuration.md)).

## Good to know

- **Each user sees only their own clips.** A clip is tied to the Jellyfin user who made it.
- **Cliparr plays the original file** through your Jellyfin server, without transcoding. Browsers only play some formats (H.264 video with AAC audio in an MP4 is safe). If a video does not play in the web interface, try the Android or Linux app, which handle more formats.
- **Deleting an item from Jellyfin** leaves its clips in your list, but they can no longer be played.
- **Preview pictures** are made when you first look at a card and then kept, so the first view of a new bookmark takes a moment. If a picture cannot be made, the card shows the movie poster instead.
