// Package config loads the server configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// Config is the runtime configuration of the Cliparr server.
type Config struct {
	// JellyfinURL is the base URL of the Jellyfin server, without trailing slash.
	JellyfinURL string
	// JellyfinPublicURL is the address of Jellyfin as seen from users' browsers.
	// Links that open Jellyfin use it; it defaults to JellyfinURL.
	JellyfinPublicURL string
	// FFmpeg is the ffmpeg program used to make clip thumbnails ("off" disables them).
	FFmpeg string
	// Listen is the address the HTTP server binds to.
	Listen string
	// DataDir is the directory holding the SQLite database.
	DataDir string
	// ProxyAuth enables single sign-on through a reverse proxy such as
	// Authelia. The zero value means disabled.
	ProxyAuth ProxyAuth
}

// ProxyAuth lets a trusted reverse proxy tell Cliparr who the user is.
type ProxyAuth struct {
	// UserHeader is the request header carrying the Jellyfin username, e.g. "Remote-User".
	UserHeader string
	// TrustedProxies are the networks allowed to set UserHeader.
	TrustedProxies []netip.Prefix
	// JellyfinAPIKey lets Cliparr act for users that never typed a password.
	JellyfinAPIKey string
}

// Enabled reports whether proxy authentication is configured.
func (p ProxyAuth) Enabled() bool { return p.UserHeader != "" }

// Trusts reports whether addr belongs to one of the trusted proxy networks.
func (p ProxyAuth) Trusts(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range p.TrustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// FromEnv builds a Config from the given lookup function (usually os.Getenv).
func FromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		JellyfinURL: strings.TrimRight(getenv("CLIPARR_JELLYFIN_URL"), "/"),
		Listen:      withDefault(getenv("CLIPARR_LISTEN"), ":8080"),
		DataDir:     withDefault(getenv("CLIPARR_DATA_DIR"), "./data"),
		FFmpeg:      withDefault(getenv("CLIPARR_FFMPEG"), "ffmpeg"),
	}
	if cfg.JellyfinURL == "" {
		return Config{}, errors.New("CLIPARR_JELLYFIN_URL is required")
	}
	if err := checkHTTPURL("CLIPARR_JELLYFIN_URL", cfg.JellyfinURL); err != nil {
		return Config{}, err
	}
	cfg.JellyfinPublicURL = withDefault(strings.TrimRight(getenv("CLIPARR_JELLYFIN_PUBLIC_URL"), "/"), cfg.JellyfinURL)
	if err := checkHTTPURL("CLIPARR_JELLYFIN_PUBLIC_URL", cfg.JellyfinPublicURL); err != nil {
		return Config{}, err
	}
	proxy, err := proxyAuthFromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg.ProxyAuth = proxy
	return cfg, nil
}

func checkHTTPURL(name, value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s %q is not a valid http(s) URL", name, value)
	}
	return nil
}

func proxyAuthFromEnv(getenv func(string) string) (ProxyAuth, error) {
	p := ProxyAuth{
		UserHeader:     strings.TrimSpace(getenv("CLIPARR_PROXY_AUTH_HEADER")),
		JellyfinAPIKey: getenv("CLIPARR_JELLYFIN_API_KEY"),
	}
	trusted := getenv("CLIPARR_PROXY_AUTH_TRUSTED")
	if !p.Enabled() {
		if trusted != "" || p.JellyfinAPIKey != "" {
			return ProxyAuth{}, errors.New("CLIPARR_PROXY_AUTH_TRUSTED and CLIPARR_JELLYFIN_API_KEY need CLIPARR_PROXY_AUTH_HEADER")
		}
		return ProxyAuth{}, nil
	}
	if p.JellyfinAPIKey == "" {
		return ProxyAuth{}, errors.New("CLIPARR_JELLYFIN_API_KEY is required when CLIPARR_PROXY_AUTH_HEADER is set")
	}
	for _, entry := range strings.Split(trusted, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := parsePrefix(entry)
		if err != nil {
			return ProxyAuth{}, fmt.Errorf("CLIPARR_PROXY_AUTH_TRUSTED: %q is not an IP address or CIDR network", entry)
		}
		p.TrustedProxies = append(p.TrustedProxies, prefix)
	}
	if len(p.TrustedProxies) == 0 {
		return ProxyAuth{}, errors.New("CLIPARR_PROXY_AUTH_TRUSTED is required when CLIPARR_PROXY_AUTH_HEADER is set")
	}
	return p, nil
}

// parsePrefix accepts "10.0.0.0/8" as well as a single address like "172.18.0.2".
func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
