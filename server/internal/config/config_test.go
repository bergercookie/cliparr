package config

import (
	"net/netip"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnv(t *testing.T) {
	cfg, err := FromEnv(env(map[string]string{"CLIPARR_JELLYFIN_URL": "http://jf:8096/"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JellyfinURL != "http://jf:8096" || cfg.Listen != ":8080" || cfg.DataDir != "./data" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.JellyfinPublicURL != "http://jf:8096" || cfg.FFmpeg != "ffmpeg" {
		t.Errorf("unexpected defaults: public %q, ffmpeg %q", cfg.JellyfinPublicURL, cfg.FFmpeg)
	}
}

func TestPublicURLCanDifferFromTheInternalOne(t *testing.T) {
	cfg, err := FromEnv(env(map[string]string{
		"CLIPARR_JELLYFIN_URL":        "http://jellyfin:8096",
		"CLIPARR_JELLYFIN_PUBLIC_URL": "https://jellyfin.example.com/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JellyfinURL != "http://jellyfin:8096" || cfg.JellyfinPublicURL != "https://jellyfin.example.com" {
		t.Errorf("unexpected URLs: %q and %q", cfg.JellyfinURL, cfg.JellyfinPublicURL)
	}
	if _, err := FromEnv(env(map[string]string{
		"CLIPARR_JELLYFIN_URL":        "http://jellyfin:8096",
		"CLIPARR_JELLYFIN_PUBLIC_URL": "jellyfin.example.com",
	})); err == nil {
		t.Error("expected an error for a public URL without a scheme")
	}
}

func TestFromEnvRejectsBadURL(t *testing.T) {
	for _, bad := range []string{"", "jf:8096", "ftp://jf", "http://"} {
		if _, err := FromEnv(env(map[string]string{"CLIPARR_JELLYFIN_URL": bad})); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestProxyAuthFromEnv(t *testing.T) {
	cfg, err := FromEnv(env(map[string]string{
		"CLIPARR_JELLYFIN_URL":       "http://jf:8096",
		"CLIPARR_PROXY_AUTH_HEADER":  "Remote-User",
		"CLIPARR_PROXY_AUTH_TRUSTED": "172.18.0.0/16, 10.0.0.5",
		"CLIPARR_JELLYFIN_API_KEY":   "key",
	}))
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.ProxyAuth
	if !p.Enabled() || len(p.TrustedProxies) != 2 {
		t.Fatalf("unexpected proxy config: %+v", p)
	}
	for addr, want := range map[string]bool{"172.18.3.4": true, "10.0.0.5": true, "10.0.0.6": false, "::ffff:172.18.0.9": true} {
		if got := p.Trusts(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Trusts(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestProxyAuthRequiresAllSettings(t *testing.T) {
	base := map[string]string{"CLIPARR_JELLYFIN_URL": "http://jf:8096"}
	cases := map[string]map[string]string{
		"header without key":     {"CLIPARR_PROXY_AUTH_HEADER": "Remote-User", "CLIPARR_PROXY_AUTH_TRUSTED": "10.0.0.0/8"},
		"header without proxies": {"CLIPARR_PROXY_AUTH_HEADER": "Remote-User", "CLIPARR_JELLYFIN_API_KEY": "k"},
		"bad network":            {"CLIPARR_PROXY_AUTH_HEADER": "Remote-User", "CLIPARR_JELLYFIN_API_KEY": "k", "CLIPARR_PROXY_AUTH_TRUSTED": "nope"},
		"key without header":     {"CLIPARR_JELLYFIN_API_KEY": "k"},
	}
	for name, extra := range cases {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		if _, err := FromEnv(env(m)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
