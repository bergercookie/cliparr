# Cliparr

[![CI](https://github.com/bergercookie/cliparr/actions/workflows/ci.yml/badge.svg)](https://github.com/bergercookie/cliparr/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/bergercookie/cliparr/graph/badge.svg)](https://codecov.io/gh/bergercookie/cliparr)
[![Latest release](https://img.shields.io/github/v/release/bergercookie/cliparr)](https://github.com/bergercookie/cliparr/releases/latest)

Save the moments of your Jellyfin movies and shows that you want to see again: a motivating speech, a perfect one-liner, a scene that just works. Cliparr keeps them in one place, with your own notes and tags, and plays each one back on a loop.

- Log in with your existing Jellyfin account. Nothing new to set up for your users.
- Pick any movie or episode, mark the start and end while watching, add a quote and a note.
- Search and filter all your clips from one page.
- Self-hosted: one small container, one SQLite file.
- Works with single sign-on (Authelia and similar) behind a reverse proxy.
- Web interface (served by the container), Android app, Linux desktop app.

Cliparr only stores *where* a clip is (item, start, end) plus your text. The video itself always streams from your Jellyfin server.

## Quick start

You need a running Jellyfin server and Docker with Compose. Save this as `compose.yaml`, set your Jellyfin address, and start it:

```yaml
services:
  cliparr:
    image: ghcr.io/bergercookie/cliparr:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      CLIPARR_JELLYFIN_URL: http://192.168.1.10:8096 # how this container reaches Jellyfin
    volumes:
      - cliparr-data:/data

volumes:
  cliparr-data:
```

```sh
docker compose up -d
```

Open <http://localhost:8080> and log in with your Jellyfin username and password.

## Documentation

| I want to...                                   | Read                                              |
| ---------------------------------------------- | ------------------------------------------------- |
| Install and use Cliparr                        | [Getting started](docs/getting-started.md)        |
| Look up a setting                              | [Configuration](docs/configuration.md)            |
| Serve it over HTTPS with Caddy                 | [Reverse proxy and HTTPS](docs/reverse-proxy.md)  |
| Use Authelia or another SSO provider           | [Single sign-on](docs/sso.md)                     |
| Install the Android or Linux app               | [Apps](docs/apps.md)                              |
| Fix a problem                                  | [Troubleshooting](docs/troubleshooting.md)        |
| Understand how it works                        | [How it works](docs/how-it-works.md)              |
| Hack on Cliparr                                | [Development](docs/development.md)                |
| Cut a release                                  | [Releasing](docs/releasing.md)                    |

The same pages are published at <https://bergercookie.github.io/cliparr/>.

## Status

Cliparr is young. Planned next: a browser extension that adds a "save this moment" button to the Jellyfin web player.
