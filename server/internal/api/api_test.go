package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
	"github.com/bergercookie/cliparr/server/internal/jellyfin"
	"github.com/bergercookie/cliparr/server/internal/store"
)

// testEnv is a Cliparr server wired to the fake Jellyfin.
type testEnv struct {
	t    *testing.T
	url  string
	fake *fakejellyfin.Server // the fake Jellyfin behind Cliparr
}

func newEnv(t *testing.T, opts ...func(*api.Options)) *testEnv {
	t.Helper()
	return newEnvWithFake(t, fakejellyfin.New(), opts...)
}

func newEnvWithFake(t *testing.T, jellyfinFake *fakejellyfin.Server, opts ...func(*api.Options)) *testEnv {
	t.Helper()
	fake := httptest.NewServer(jellyfinFake)
	t.Cleanup(fake.Close)

	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	jf, err := jellyfin.New(fake.URL, "test-device", "test")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	o := api.Options{Store: st, Jellyfin: jf, Version: "test", Log: log}
	for _, opt := range opts {
		opt(&o)
	}
	srv := httptest.NewServer(api.New(o))
	t.Cleanup(srv.Close)
	return &testEnv{t: t, url: srv.URL, fake: jellyfinFake}
}

// call sends a JSON request, optionally authenticated with a bearer token, and
// decodes a JSON response into out (if non-nil). It returns the status code.
func (e *testEnv) call(method, path, token string, body, out any) int {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, e.url+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			e.t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func (e *testEnv) login(user string) string {
	e.t.Helper()
	var resp struct{ Token string }
	if code := e.call("POST", "/api/auth/login", "", map[string]string{"username": user, "password": "pw"}, &resp); code != 200 {
		e.t.Fatalf("login %s: status %d", user, code)
	}
	return resp.Token
}

func TestLogin(t *testing.T) {
	e := newEnv(t)

	if code := e.call("POST", "/api/auth/login", "", map[string]string{"username": "alice", "password": "wrong"}, nil); code != 401 {
		t.Errorf("wrong password: got %d, want 401", code)
	}
	if code := e.call("POST", "/api/auth/login", "", map[string]string{"username": "alice"}, nil); code != 400 {
		t.Errorf("missing password: got %d, want 400", code)
	}

	token := e.login("alice")
	var me struct{ Name string }
	if code := e.call("GET", "/api/me", token, nil, &me); code != 200 || me.Name != "alice" {
		t.Errorf("me: code %d, body %+v", code, me)
	}
	if code := e.call("POST", "/api/auth/logout", token, nil, nil); code != 204 {
		t.Errorf("logout: got %d", code)
	}
	if code := e.call("GET", "/api/me", token, nil, nil); code != 401 {
		t.Errorf("token still valid after logout: %d", code)
	}
}

func TestLoginSetsHardenedCookie(t *testing.T) {
	e := newEnv(t)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pw"})
	resp, err := http.Post(e.url+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cookies := resp.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("unexpected cookies: %+v", cookies)
	}
}

func TestProtectedEndpointsRequireLogin(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/api/me", "/api/items?q=a", "/api/clips", "/api/items/movie-rocky/stream"} {
		if code := e.call("GET", path, "", nil, nil); code != 401 {
			t.Errorf("GET %s without login: got %d, want 401", path, code)
		}
	}
}

func TestRejectsNonJSONBody(t *testing.T) {
	e := newEnv(t)
	resp, err := http.Post(e.url+"/api/auth/login", "application/x-www-form-urlencoded", strings.NewReader("username=a&password=b"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("got %d, want 415", resp.StatusCode)
	}
}

func TestSearchRespectsJellyfinPermissions(t *testing.T) {
	e := newEnv(t)
	var alice, bob []struct{ ID string }
	e.call("GET", "/api/items?q=secret", e.login("alice"), nil, &alice)
	e.call("GET", "/api/items?q=secret", e.login("bob"), nil, &bob)
	if len(alice) != 1 || len(bob) != 0 {
		t.Errorf("alice sees %d, bob sees %d secret items; want 1 and 0", len(alice), len(bob))
	}
}

func TestClipCRUD(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")

	type clip struct {
		ID      int64    `json:"id"`
		Title   string   `json:"title"`
		StartMs int64    `json:"startMs"`
		EndMs   int64    `json:"endMs"`
		Quote   string   `json:"quote"`
		Tags    []string `json:"tags"`
	}
	var created clip
	code := e.call("POST", "/api/clips", token, map[string]any{
		"itemId": "movie-rocky", "startMs": 1000, "endMs": 9000,
		"quote": "  Yo Adrian!  ", "tags": []string{"Motivation", "motivation", " ", "Classic"},
	}, &created)
	if code != 201 || created.Title != "Rocky (1976)" || created.Quote != "Yo Adrian!" {
		t.Fatalf("create: code %d, clip %+v", code, created)
	}
	if strings.Join(created.Tags, ",") != "motivation,classic" {
		t.Errorf("tags not normalized: %v", created.Tags)
	}

	path := "/api/clips/" + itoa(created.ID)
	var updated clip
	code = e.call("PUT", path, token, map[string]any{"startMs": 2000, "endMs": 8000, "quote": "Edited"}, &updated)
	if code != 200 || updated.Quote != "Edited" || updated.StartMs != 2000 || updated.Title != "Rocky (1976)" {
		t.Errorf("update: code %d, clip %+v", code, updated)
	}

	var list []clip
	e.call("GET", "/api/clips?q=edited", token, nil, &list)
	if len(list) != 1 {
		t.Errorf("search found %d clips, want 1", len(list))
	}

	if code := e.call("DELETE", path, token, nil, nil); code != 204 {
		t.Errorf("delete: got %d", code)
	}
	if code := e.call("GET", path, token, nil, nil); code != 404 {
		t.Errorf("deleted clip still readable: %d", code)
	}
}

func TestCreateClipValidation(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")
	cases := map[string]map[string]any{
		"missing item":     {"startMs": 0, "endMs": 10},
		"negative start":   {"itemId": "movie-rocky", "startMs": -1, "endMs": 10},
		"end before start": {"itemId": "movie-rocky", "startMs": 10, "endMs": 10},
		"past the end":     {"itemId": "movie-rocky", "startMs": 0, "endMs": 999_999_999},
		"comma in tag":     {"itemId": "movie-rocky", "startMs": 0, "endMs": 10, "tags": []string{"a,b"}},
	}
	for name, body := range cases {
		if code := e.call("POST", "/api/clips", token, body, nil); code != 400 {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}
}

func TestCannotClipItemUserCannotSee(t *testing.T) {
	e := newEnv(t)
	code := e.call("POST", "/api/clips", e.login("bob"), map[string]any{"itemId": "movie-secret", "startMs": 0, "endMs": 10}, nil)
	if code != 404 {
		t.Errorf("got %d, want 404", code)
	}
}

func TestClipsArePrivate(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.login("alice"), e.login("bob")
	var created struct{ ID int64 }
	e.call("POST", "/api/clips", alice, map[string]any{"itemId": "movie-rocky", "startMs": 0, "endMs": 10}, &created)

	path := "/api/clips/" + itoa(created.ID)
	if code := e.call("GET", path, bob, nil, nil); code != 404 {
		t.Errorf("bob read alice's clip: %d", code)
	}
	if code := e.call("DELETE", path, bob, nil, nil); code != 404 {
		t.Errorf("bob deleted alice's clip: %d", code)
	}
	var list []any
	e.call("GET", "/api/clips", bob, nil, &list)
	if len(list) != 0 {
		t.Errorf("bob sees %d of alice's clips", len(list))
	}
}

func TestStreamSupportsRangeRequests(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")
	req, _ := http.NewRequest("GET", e.url+"/api/items/movie-rocky/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Range", "bytes=10-19")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent || len(body) != 10 {
		t.Errorf("got status %d with %d bytes, want 206 with 10 bytes", resp.StatusCode, len(body))
	}
}

func TestStreamIsDeniedForHiddenItems(t *testing.T) {
	e := newEnv(t)
	req, _ := http.NewRequest("GET", e.url+"/api/items/movie-secret/stream", nil)
	req.Header.Set("Authorization", "Bearer "+e.login("bob"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
