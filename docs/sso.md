# Single sign-on (Authelia and similar)

By default, people log in to Cliparr with their Jellyfin username and password. This page covers the case where a reverse proxy with single sign-on (SSO) sits in front instead, for example [Authelia](https://www.authelia.com/), Authentik or oauth2-proxy.

You may want that because:

- You already use SSO and do not want a second login prompt.
- Your Jellyfin users sign in through an SSO plugin and have **no Jellyfin password**, so the normal login cannot work for them.
- You want two-factor authentication in front of Cliparr.

## How it works

1. The proxy asks Authelia whether the visitor is logged in. If not, Authelia shows its login page.
2. If yes, the proxy forwards the request to Cliparr and adds a header with the user's name, `Remote-User: alice`.
3. Cliparr trusts that header, but **only** when the request comes from your proxy's address. It looks up the Jellyfin user named `alice` and shows `alice`'s clips.

Because Cliparr never sees a password in this mode, it uses a Jellyfin **API key** to act for the user. An API key can see everything, so Cliparr checks each movie or episode against the user's own Jellyfin permissions before streaming it. A user can never open something Jellyfin would hide from them.

**Requirement:** the Authelia username must match the Jellyfin username (capitalisation is ignored).

## Set it up

This assumes you followed [Reverse proxy and HTTPS](reverse-proxy.md) and have Authelia running at `auth.example.com`.

### 1. Create a Jellyfin API key

In Jellyfin: **Dashboard → API Keys → +**, name it `cliparr`, and copy the key.

### 2. Give a static network to your proxy and Cliparr

Cliparr must know your proxy's address. Pin the network in Compose so the address does not change:

```yaml
networks:
  default:
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

Add this once at the top level of your `compose.yaml`. Every container in the file is now in `172.20.0.0/16`.

### 3. Configure Cliparr

```yaml
  cliparr:
    image: ghcr.io/bergercookie/cliparr:latest
    restart: unless-stopped
    environment:
      CLIPARR_JELLYFIN_URL: http://192.168.1.10:8096
      CLIPARR_JELLYFIN_API_KEY: paste-your-key-here
      CLIPARR_PROXY_AUTH_HEADER: Remote-User
      CLIPARR_PROXY_AUTH_TRUSTED: 172.20.0.0/16
    volumes:
      - cliparr-data:/data
```

`CLIPARR_PROXY_AUTH_TRUSTED` accepts a comma-separated list of addresses or networks. Make it as narrow as you can: only your proxy should be in it. Cliparr refuses to start if the header is set without the API key or the trusted list.

### 4. Configure Caddy

```
cliparr.example.com {
	forward_auth authelia:9091 {
		uri /api/authz/forward-auth
		copy_headers Remote-User Remote-Groups Remote-Name Remote-Email
	}
	reverse_proxy cliparr:8080
}
```

`copy_headers` replaces any `Remote-User` header the visitor sent themselves with the one from Authelia, so it cannot be faked.

### 5. Allow the domain in Authelia

In Authelia's `configuration.yml`, add an access rule for the domain:

```yaml
access_control:
  default_policy: deny
  rules:
    - domain: cliparr.example.com
      policy: one_factor # or two_factor
```

Authelia 4.38 or newer is assumed (the `/api/authz/forward-auth` endpoint). Restart Authelia and Caddy.

### 6. Test it

Open `https://cliparr.example.com` in a private window. You should be sent to Authelia, and after logging in land directly on your clips page without a Jellyfin login. The top-right shows your username instead of a logout button, because logging out is Authelia's job.

If you see "your SSO username does not match any Jellyfin user", create or rename the user so both names match.

## The apps cannot do SSO

The Android and Linux apps cannot complete an Authelia login. Give them a second host name that skips Authelia and uses the normal Jellyfin password login:

```
app.cliparr.example.com {
	request_header -Remote-User
	reverse_proxy cliparr:8080
}
```

**The `request_header -Remote-User` line is essential.** Without it, anyone could send `Remote-User: alice` to this host name, and because the request comes from your trusted proxy, Cliparr would believe it. The automated test suite checks this setup.

This host name exposes Cliparr's Jellyfin password login to the internet, as Jellyfin's own login page is. If you do not use the apps, do not create it. Users without a Jellyfin password cannot use the apps.

## Security checklist

- Cliparr's port is **not** published; only the proxy can reach it.
- `CLIPARR_PROXY_AUTH_TRUSTED` contains only your proxy.
- Every proxy site that reaches Cliparr either sets `Remote-User` from Authelia or removes it.
- The API key is kept secret. It grants administrator access to Jellyfin; if it leaks, revoke it in the Jellyfin dashboard and create a new one.
- Changes in Jellyfin (a deleted user, a revoked library permission) reach Cliparr within five minutes.
