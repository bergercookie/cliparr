# Reverse proxy and HTTPS

To reach Cliparr from outside your home, or just to get a clean `https://cliparr.example.com` address, put a reverse proxy in front of it. This page uses [Caddy](https://caddyserver.com/), which gets and renews HTTPS certificates by itself.

Cliparr listens on plain HTTP. Never expose that port directly to the internet; let the proxy terminate HTTPS.

## Caddy with Docker Compose

Create a `Caddyfile` next to your `compose.yaml`:

```
cliparr.example.com {
	reverse_proxy cliparr:8080
}
```

Replace `cliparr.example.com` with your domain. Then extend the Compose file. Cliparr no longer publishes a port; only Caddy is reachable from outside:

```yaml
services:
  caddy:
    image: caddy:2
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config

  cliparr:
    image: ghcr.io/bergercookie/cliparr:latest
    restart: unless-stopped
    environment:
      CLIPARR_JELLYFIN_URL: http://192.168.1.10:8096
    volumes:
      - cliparr-data:/data

volumes:
  caddy-data:
  caddy-config:
  cliparr-data:
```

Run `docker compose up -d`. Caddy requests a certificate on the first visit to your domain.

Keep the `caddy-data` volume: it holds your certificates and account key. Losing it makes Caddy request new certificates, which can hit Let's Encrypt rate limits.

## Getting a certificate

Caddy needs to prove that you own the domain. Pick the option that matches your setup.

**Your domain points at your home and you can forward ports 80 and 443 to the Caddy host.** This is the default and needs no extra configuration. Make sure the DNS record points at your home IP.

**You only use Cliparr inside your home network, but want a trusted certificate.** Use a DNS challenge: Caddy proves ownership by creating a DNS record at your DNS provider, so no port needs to be open. This needs a Caddy build that includes your provider's plugin (for example `caddy-dns/cloudflare`); see the [Caddy documentation](https://caddyserver.com/docs/automatic-https#dns-challenge). Point the domain at the LAN address of your Caddy host in your DNS.

**You only use Cliparr inside your home network and a certificate warning is fine.** Tell Caddy to use its own certificate authority:

```
cliparr.example.com {
	tls internal
	reverse_proxy cliparr:8080
}
```

Browsers warn until you install Caddy's root certificate on each device (it is in the `caddy-data` volume under `caddy/pki/authorities/local/root.crt`). The Android app has the same limitation.

## Check that it works

```sh
curl https://cliparr.example.com/api/info
# {"name":"cliparr","version":"..."}
```

Log in on the web page. Playing a clip exercises streaming and seeking through the proxy; Caddy handles both without extra settings.

### If the browser shows a TLS error

An error such as `SSL_ERROR_INTERNAL_ERROR_ALERT` in Firefox, or `ERR_SSL_PROTOCOL_ERROR` in Chrome, while Cliparr's log shows nothing, means the request never reached Cliparr: Caddy had no certificate for the name you opened. Ask Caddy's server directly, using the exact name from the address bar:

```sh
openssl s_client -connect cliparr.example.com:443 -servername cliparr.example.com </dev/null 2>&1 | grep -E "alert|subject="
```

A line `subject=…` means Caddy has a certificate for that name. `tlsv1 alert internal error` means it has none. Then check, in this order:

1. The Caddyfile has a block for exactly that name (`cliparr.example.com { … }`) and Caddy was reloaded or restarted after you added it.
2. A wildcard certificate covers only one more label: `*.example.com` fits `cliparr.example.com` but not `cliparr.home.example.com`. A name with an extra label needs a certificate of its own, or a wildcard for `*.home.example.com`.
3. Getting a certificate worked: `docker compose logs caddy | grep -iE "obtain|error"` shows what Let's Encrypt or your DNS provider said.

Caddy's debug log line `no OCSP stapling … no OCSP server specified in certificate` is harmless. Let's Encrypt no longer puts an OCSP address into its certificates, so Caddy has nothing to staple. It also shows that Caddy has a certificate for that name.

## Notes

- **Logins stay secure automatically.** Caddy tells Cliparr the connection was HTTPS (`X-Forwarded-Proto`), and Cliparr then marks its session cookie `Secure`.
- **Do not publish Cliparr's own port** (`ports:` on the `cliparr` service) once the proxy is in place, especially if you use [single sign-on](sso.md).
- **A sub-path such as `example.com/cliparr` is not supported by the web interface.** Use a dedicated host name.
- **No built-in login rate limiting.** If you expose Cliparr publicly with password login, use a strong Jellyfin password, or put it behind [single sign-on](sso.md).
