# Configuration

Cliparr is configured with environment variables. Only one is required.

## Basic settings

| Variable               | Default  | Meaning                                                                                      |
| ---------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `CLIPARR_JELLYFIN_URL` | required | Address of your Jellyfin server, e.g. `http://jellyfin:8096`. Must be reachable from Cliparr. |
| `CLIPARR_LISTEN`       | `:8080`  | Address and port Cliparr listens on inside the container.                                    |
| `CLIPARR_DATA_DIR`     | `/data` in the image | Directory for the database. Mount a volume here.                                 |

## Single sign-on settings

Only needed when a reverse proxy such as Authelia logs users in. See [Single sign-on](sso.md) for the full setup. Set all three or none.

| Variable                     | Example                  | Meaning                                                                                       |
| ---------------------------- | ------------------------ | --------------------------------------------------------------------------------------------- |
| `CLIPARR_PROXY_AUTH_HEADER`  | `Remote-User`            | Request header in which your proxy sends the username.                                        |
| `CLIPARR_PROXY_AUTH_TRUSTED` | `172.20.0.0/16`          | Comma-separated IP addresses or networks of your proxy. The header is ignored from anywhere else. |
| `CLIPARR_JELLYFIN_API_KEY`   | (from Jellyfin)          | Lets Cliparr act for users who never typed a Jellyfin password.                               |

## Log output

Cliparr logs one line per request (method, path, status, duration) to standard output. It never logs passwords, tokens or search terms. View it with `docker compose logs cliparr`.

## Upgrading

```sh
docker compose pull
docker compose up -d
```

The database is upgraded automatically on start. To try the latest development build instead of the newest release, use the `nightly` tag: `ghcr.io/bergercookie/cliparr:nightly`.
