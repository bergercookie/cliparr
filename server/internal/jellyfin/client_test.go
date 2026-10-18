package jellyfin_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

func newClient(t *testing.T) *jellyfin.Client {
	t.Helper()
	fake := httptest.NewServer(fakejellyfin.New())
	t.Cleanup(fake.Close)
	c, err := jellyfin.New(fake.URL, "test-device", "test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAuthenticate(t *testing.T) {
	c := newClient(t)

	sess, err := c.Authenticate(t.Context(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if sess.User.Name != "alice" || sess.User.ID != "id-alice" || sess.Token == "" {
		t.Errorf("unexpected session: %+v", sess)
	}

	if _, err := c.Authenticate(t.Context(), "alice", "wrong"); !errors.Is(err, jellyfin.ErrUnauthorized) {
		t.Errorf("wrong password: got %v, want ErrUnauthorized", err)
	}
}

func TestSearchBuildsReadableTitles(t *testing.T) {
	c := newClient(t)
	sess, _ := c.Authenticate(t.Context(), "alice", "pw")

	titles := map[string]string{}
	for _, query := range []string{"rocky", "pilot"} {
		items, err := c.Search(t.Context(), sess.Token, sess.User.ID, query, 10)
		if err != nil || len(items) != 1 {
			t.Fatalf("search %q: %v, %d items", query, err, len(items))
		}
		titles[query] = items[0].Title
	}
	if titles["rocky"] != "Rocky (1976)" {
		t.Errorf("movie title: %q", titles["rocky"])
	}
	if titles["pilot"] != "Fake Show S01E01 - Pilot" {
		t.Errorf("episode title: %q", titles["pilot"])
	}
}

// Jellyfin keeps the file name, year included, for media it could not identify.
func TestTitleDoesNotRepeatTheYearOfAnUnidentifiedFile(t *testing.T) {
	jf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Items":[
			{"Id":"a","Name":"Test Movie (2020)","Type":"Movie","ProductionYear":2020},
			{"Id":"b","Name":"Rocky","Type":"Movie","ProductionYear":1976},
			{"Id":"c","Name":"Blade Runner 2049","Type":"Movie","ProductionYear":2017}]}`))
	}))
	t.Cleanup(jf.Close)
	c, err := jellyfin.New(jf.URL, "test-device", "test")
	if err != nil {
		t.Fatal(err)
	}

	items, err := c.Search(t.Context(), "token", "user", "x", 10)
	if err != nil || len(items) != 3 {
		t.Fatalf("search: %v, %d items", err, len(items))
	}
	want := []string{"Test Movie (2020)", "Rocky (1976)", "Blade Runner 2049 (2017)"}
	for i, item := range items {
		if item.Title != want[i] {
			t.Errorf("title %d: got %q, want %q", i, item.Title, want[i])
		}
	}
}

func TestItemConvertsRuntimeToMilliseconds(t *testing.T) {
	c := newClient(t)
	sess, _ := c.Authenticate(t.Context(), "alice", "pw")

	item, err := c.Item(t.Context(), sess.Token, sess.User.ID, "movie-rocky")
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(120 * 60 * 1000); item.RuntimeMs != want {
		t.Errorf("runtime: got %d ms, want %d", item.RuntimeMs, want)
	}
}

func TestItemNotVisibleToUser(t *testing.T) {
	c := newClient(t)
	sess, _ := c.Authenticate(t.Context(), "bob", "pw")

	if _, err := c.Item(t.Context(), sess.Token, sess.User.ID, "movie-secret"); !errors.Is(err, jellyfin.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestRejectedTokenIsReported(t *testing.T) {
	c := newClient(t)
	if _, err := c.Item(t.Context(), "bogus", "id-alice", "movie-rocky"); !errors.Is(err, jellyfin.ErrUnauthorized) {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}

func TestUserByName(t *testing.T) {
	c := newClient(t)

	user, err := c.UserByName(t.Context(), fakejellyfin.APIKey, "BOB")
	if err != nil || user.Name != "bob" || user.ID != "id-bob" {
		t.Errorf("got %+v, %v", user, err)
	}
	if _, err := c.UserByName(t.Context(), fakejellyfin.APIKey, "nobody"); !errors.Is(err, jellyfin.ErrNotFound) {
		t.Errorf("unknown user: got %v, want ErrNotFound", err)
	}
	if _, err := c.UserByName(t.Context(), "not-the-key", "bob"); !errors.Is(err, jellyfin.ErrUnauthorized) {
		t.Errorf("wrong key: got %v, want ErrUnauthorized", err)
	}
}

func TestUserByToken(t *testing.T) {
	c := newClient(t)

	user, err := c.UserByToken(t.Context(), "token-alice")
	if err != nil || user.Name != "alice" || user.ID != "id-alice" {
		t.Errorf("got %+v, %v", user, err)
	}
	for _, bad := range []string{"nope", fakejellyfin.APIKey} {
		if _, err := c.UserByToken(t.Context(), bad); !errors.Is(err, jellyfin.ErrUnauthorized) {
			t.Errorf("token %q: got %v, want ErrUnauthorized", bad, err)
		}
	}
}
