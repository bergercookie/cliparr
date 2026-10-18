# Configuration

Cliparr is configured with environment variables. Only one is required.

## Basic settings

| Variable               | Default  | Meaning                                                                                      |
| ---------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `CLIPARR_JELLYFIN_URL` | required | Address of your Jellyfin server, e.g. `http://jellyfin:8096`. Must be reachable from Cliparr. |
| `CLIPARR_JELLYFIN_PUBLIC_URL` | same as `CLIPARR_JELLYFIN_URL` | Address of Jellyfin as you type it in your browser, e.g. `https://jellyfin.example.com`. Used for "Open in Jellyfin web" links. Set it when `CLIPARR_JELLYFIN_URL` is an internal name like `http://jellyfin:8096`. Cliparr warns at startup when you have not set it and the Jellyfin address looks like one that only the server can use (`localhost` or a name without dots). |
| `CLIPARR_LISTEN`       | `:8080`  | Address and port Cliparr listens on inside the container.                                    |
| `CLIPARR_DATA_DIR`     | `/data` in the image | Directory for the database and the preview pictures. Mount a volume here.        |
| `PUID` | `65532` | The user number that owns the data folder and runs Cliparr. Use the output of `id -u`. Cannot be 0. See [Data folder and permissions](#data-folder-and-permissions). |
| `PGID` | same as `PUID` | The group number, from `id -g`. |
| `CLIPARR_FFMPEG`       | `ffmpeg` | Program that makes the preview pictures. The Docker image includes it. Set to `off` to show movie posters instead. |

The preview pictures are small JPEG files in the `thumbnails` folder of the data directory. They can be deleted at any time; Cliparr makes them again when needed. If you run the plain binary instead of the Docker image, install `ffmpeg` so previews work.

## Data folder and permissions

Cliparr keeps its database and the preview pictures in `/data` inside the container. You want those files to belong to you, not to some account inside the container, so that you can copy and back them up without `sudo`.

Set `PUID` and `PGID` to your own user and group, from `id -u` and `id -g`, and mount a folder of yours as `/data`:

```yaml
    environment:
      PUID: "1000"
      PGID: "1000"
    volumes:
      - ./cliparr-data:/data
```

The container starts as root for a moment. Before it opens the database or a single connection, Cliparr gives `/data` and everything in it to `PUID:PGID`, and then gives up root for good: from then on it runs as that user, which you can see in the `cliparr started` log line (`uid=1000 gid=1000`). It does not matter whether Docker created the folder (it does so as root when the folder is missing) or an earlier run left files owned by someone else: Cliparr fixes it at every start, and changing `PUID` later works the same way. It never follows a link out of the data folder, so a link inside it cannot make it change a file elsewhere.

- **Only a folder of your own helps with backups.** A named Docker volume (`cliparr-data:/data` with a `volumes:` entry) works too, but it lives under `/var/lib/docker`, which only root can read, whoever owns the files.
- **Back up with Cliparr stopped.** The database is a live SQLite file with side files. Run `docker compose stop`, copy the folder, then `docker compose start`.
- **Without `PUID` and `PGID`** the files belong to user and group `65532`, the account the image has always used.

### Never root at all: `user:`

Handing the folder over needs root and three Linux capabilities (`CHOWN`, `SETUID`, `SETGID`). If you would rather never start as root, or you drop all capabilities (`cap_drop: [ALL]`), or the folder is on a file system that refuses `chown` (NFS with root squash, for instance), set `user:` yourself and make the folder yours first. `PUID` and `PGID` are then ignored.

```yaml
services:
  cliparr:
    user: "1000:1000" # `id -u` and `id -g`
    volumes:
      - ./cliparr-data:/data # create it first: mkdir cliparr-data
```

Docker creates a missing folder as root, and then Cliparr, running as you, cannot write to it. It stops at startup and the log says `the data folder /data is not writable by this process (uid …, gid …)`. Version 0.1.0 shows `unable to open database file (14)` for the same reason, and does not have `PUID` and `PGID`.

## Single sign-on settings

Only needed when a reverse proxy such as Authelia logs users in. See [Single sign-on](sso.md) for the full setup. Set all three or none.

| Variable                     | Example                  | Meaning                                                                                       |
| ---------------------------- | ------------------------ | --------------------------------------------------------------------------------------------- |
| `CLIPARR_PROXY_AUTH_HEADER`  | `Remote-User`            | Request header in which your proxy sends the username.                                        |
| `CLIPARR_PROXY_AUTH_TRUSTED` | `172.20.0.0/16`          | Comma-separated IP addresses or networks of your proxy. The header is ignored from anywhere else. |
| `CLIPARR_JELLYFIN_API_KEY`   | (from Jellyfin)          | Lets Cliparr act for users who never typed a Jellyfin password.                               |

## Log output

On startup Cliparr checks whether Jellyfin answers. It logs `jellyfin reachable` with Jellyfin's version, or `cannot reach Jellyfin yet` and keeps trying every ten seconds. Then it logs one line per request (method, path, status, duration) to standard output. It never logs passwords, tokens or search terms. View it with `docker compose logs cliparr`.

## Upgrading

```sh
docker compose pull
docker compose up -d
```

The database is upgraded automatically on start. To try the latest development build instead of the newest release, use the `nightly` tag: `ghcr.io/bergercookie/cliparr:nightly`.
