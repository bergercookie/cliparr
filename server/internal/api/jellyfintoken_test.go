package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
)

// asJellyfin sends a JSON request authenticated with "Authorization: Jellyfin <token>".
func (e *testEnv) asJellyfin(method, path, token string, body, out any) int {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, e.url+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Jellyfin "+token)
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

func TestJellyfinTokenActsAsThatUser(t *testing.T) {
	e := newEnv(t)
	var me struct {
		Name     string
		ViaProxy bool
	}
	if code := e.asJellyfin("GET", "/api/me", "token-alice", nil, &me); code != 200 || me.Name != "alice" || me.ViaProxy {
		t.Errorf("status %d, me %+v", code, me)
	}
}

func TestJellyfinTokenCanSaveAndListClips(t *testing.T) {
	e := newEnv(t)
	var created struct {
		ID    int64
		Title string
	}
	code := e.asJellyfin("POST", "/api/clips", "token-alice", map[string]any{"itemId": "movie-rocky", "startMs": 1000, "endMs": 5000}, &created)
	if code != 201 || created.Title != "Rocky (1976)" {
		t.Fatalf("create: status %d, clip %+v", code, created)
	}

	// The same user logged in with a password sees the clip the extension saved.
	var list []struct{ ID int64 }
	e.call("GET", "/api/clips", e.login("alice"), nil, &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Errorf("alice sees %+v", list)
	}
}

func TestJellyfinTokenUsersStayIsolated(t *testing.T) {
	e := newEnv(t)
	id := e.createClip(e.login("alice"), 1000, 2000)

	if code := e.asJellyfin("GET", clipPath(id, ""), "token-bob", nil, nil); code != 404 {
		t.Errorf("bob read alice's clip: status %d", code)
	}
	// And Jellyfin's own rules apply: bob cannot bookmark what he cannot see.
	code := e.asJellyfin("POST", "/api/clips", "token-bob", map[string]any{"itemId": "movie-secret", "startMs": 0, "endMs": 10}, nil)
	if code != 404 {
		t.Errorf("bob bookmarked a hidden movie: status %d", code)
	}
}

func TestJellyfinTokenMustBeValid(t *testing.T) {
	e := newEnv(t)
	for name, token := range map[string]string{
		"unknown token": "token-mallory",
		"garbage":       "x",
		"empty":         "",
		// An API key has no user. Accepting it would let the key's holder act as anyone.
		"administrator API key": fakejellyfin.APIKey,
	} {
		if code := e.asJellyfin("GET", "/api/me", token, nil, nil); code != 401 {
			t.Errorf("%s: status %d, want 401", name, code)
		}
	}
}
