package config

import (
	"net/netip"
	"strings"
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

func TestPublicURLAdvice(t *testing.T) {
	tests := []struct {
		name, jellyfin, public string
		wantAdvice             bool
	}{
		{"a Compose service name", "http://jellyfin:8096", "", true},
		{"localhost", "http://localhost:8096", "", true},
		{"a loopback address", "http://127.0.0.1:8096", "", true},
		{"a LAN address works at home", "http://192.168.1.10:8096", "", false},
		{"a domain name", "https://jellyfin.example.com", "", false},
		{"the public address is set", "http://jellyfin:8096", "https://jellyfin.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := FromEnv(env(map[string]string{"CLIPARR_JELLYFIN_URL": tt.jellyfin, "CLIPARR_JELLYFIN_PUBLIC_URL": tt.public}))
			if err != nil {
				t.Fatal(err)
			}
			advice := cfg.PublicURLAdvice()
			if (advice != "") != tt.wantAdvice {
				t.Errorf("advice = %q, want advice: %v", advice, tt.wantAdvice)
			}
			if tt.wantAdvice && !strings.Contains(advice, "CLIPARR_JELLYFIN_PUBLIC_URL") {
				t.Errorf("the advice should name the setting: %q", advice)
			}
		})
	}
}
