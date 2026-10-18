# Configuration

Cliparr is configured with environment variables. Only one is required.

## Basic settings

| Variable               | Default  | Meaning                                                                                      |
| ---------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `CLIPARR_JELLYFIN_URL` | required | Address of your Jellyfin server, e.g. `http://jellyfin:8096`. Must be reachable from Cliparr. |
| `CLIPARR_JELLYFIN_PUBLIC_URL` | same as `CLIPARR_JELLYFIN_URL` | Address of Jellyfin as you type it in your browser, e.g. `https://jellyfin.example.com`. Used for "Open in Jellyfin web" links. Set it when `CLIPARR_JELLYFIN_URL` is an internal name like `http://jellyfin:8096`. Cliparr warns at startup when you have not set it and the Jellyfin address looks like one that only the server can use (`localhost` or a name without dots). |
| `CLIPARR_LISTEN`       | `:8080`  | Address and port Cliparr listens on inside the container.                                    |
| `CLIPARR_DATA_DIR`     | `/data` in the image | Directory for the database and the preview pictures. Mount a volume here.        |
| `CLIPARR_FFMPEG`       | `ffmpeg` | Program that makes the preview pictures. The Docker image includes it. Set to `off` to show movie posters instead. |

The preview pictures are small JPEG files in the `thumbnails` folder of the data directory. They can be deleted at any time; Cliparr makes them again when needed. If you run the plain binary instead of the Docker image, install `ffmpeg` so previews work.

## Data folder and permissions

Cliparr keeps its database and the preview pictures in `/data` inside the container. The container does not run as root: it runs as user `65532`, and that is the user who has to be allowed to write there. It does not matter which user starts `docker compose up`.

- **A Docker volume** (`cliparr-data:/data`, as in the example) works as it is: Docker gives a new volume the owner of the image's `/data` folder.
- **A folder of your own** (`./cliparr-data:/data`) belongs to you, not to user 65532, so Cliparr cannot write to it, and neither can it if Docker had to create the folder itself, which it does as root. Create the folder first, then do one of these:
  - give it to the container's user: `sudo chown 65532:65532 cliparr-data`; or
  - run the container as yourself: add `user: "1000:1000"` to the `cliparr` service, with your own numbers from `id -u` and `id -g`.

```yaml
services:
  cliparr:
    user: "1000:1000"
    volumes:
      - ./cliparr-data:/data
```

Do not combine `user:` with a Docker volume that was created without it: the volume belongs to 65532, so the new user cannot write to it. Use a folder you own, or remove the line.

If it is wrong, Cliparr stops at startup and the log says `the data folder /data is not writable by this process (uid …, gid …)`. Version 0.1.0 shows `unable to open database file (14)` instead, for the same reason.

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
