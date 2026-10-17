package api_test

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/config"
	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
)

// withProxyAuth enables SSO through the Remote-User header for the given trusted network.
func withProxyAuth(trusted string) func(*api.Options) {
	return func(o *api.Options) {
		o.ProxyAuth = config.ProxyAuth{
			UserHeader:     "Remote-User",
			TrustedProxies: []netip.Prefix{netip.MustParsePrefix(trusted)},
			JellyfinAPIKey: fakejellyfin.APIKey,
		}
	}
}

// asUser sends a GET with the proxy header and returns status and decoded body.
func (e *testEnv) asUser(path, user string, out any) int {
	e.t.Helper()
	req, _ := http.NewRequest("GET", e.url+path, nil)
	req.Header.Set("Remote-User", user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestProxyAuthLogsInTheNamedUser(t *testing.T) {
	e := newEnv(t, withProxyAuth("127.0.0.0/8"))
	var me struct {
		Name     string
		ViaProxy bool
	}
	// Case-insensitive, because Authelia and Jellyfin may disagree on capitalisation.
	if code := e.asUser("/api/me", "ALICE", &me); code != 200 || me.Name != "alice" || !me.ViaProxy {
		t.Errorf("code %d, me %+v", code, me)
	}
}

func TestProxyAuthIgnoresHeaderFromUntrustedAddress(t *testing.T) {
	e := newEnv(t, withProxyAuth("10.0.0.0/8")) // the test client connects from 127.0.0.1
	if code := e.asUser("/api/me", "alice", nil); code != 401 {
		t.Errorf("spoofed header accepted: got %d, want 401", code)
	}
}

func TestProxyAuthRejectsUnknownUser(t *testing.T) {
	e := newEnv(t, withProxyAuth("127.0.0.0/8"))
	if code := e.asUser("/api/me", "mallory", nil); code != 403 {
		t.Errorf("got %d, want 403", code)
	}
}

func TestProxyAuthDisabledByDefault(t *testing.T) {
	e := newEnv(t)
	if code := e.asUser("/api/me", "alice", nil); code != 401 {
		t.Errorf("header accepted although proxy auth is off: got %d", code)
	}
}

func TestProxyAuthStillAllowsPasswordLogin(t *testing.T) {
	e := newEnv(t, withProxyAuth("127.0.0.0/8"))
	var me struct{ Name string }
	if code := e.call("GET", "/api/me", e.login("bob"), nil, &me); code != 200 || me.Name != "bob" {
		t.Errorf("code %d, me %+v", code, me)
	}
}

// The API key can read every item, so Cliparr must apply the user's own permissions.
func TestProxyAuthEnforcesPerUserVisibility(t *testing.T) {
	e := newEnv(t, withProxyAuth("127.0.0.0/8"))
	for _, path := range []string{"/api/items/movie-secret/stream", "/api/items/movie-secret/image"} {
		if code := e.asUser(path, "bob", nil); code != 404 {
			t.Errorf("bob GET %s: got %d, want 404", path, code)
		}
		if code := e.asUser(path, "alice", nil); code != 200 {
			t.Errorf("alice GET %s: got %d, want 200", path, code)
		}
	}
	var items []struct{ ID string }
	e.asUser("/api/items?q=secret", "bob", &items)
	if len(items) != 0 {
		t.Errorf("bob can search for the secret movie: %v", items)
	}
}
