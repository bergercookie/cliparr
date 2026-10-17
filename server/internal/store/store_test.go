package store

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDeviceIDIsStable(t *testing.T) {
	s := newStore(t)
	a, err := s.DeviceID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.DeviceID(t.Context())
	if a == "" || a != b {
		t.Errorf("device id not stable: %q vs %q", a, b)
	}
}

func TestClipLifecycle(t *testing.T) {
	s := newStore(t)
	created, err := s.CreateClip(t.Context(), Clip{
		UserID: "u1", ItemID: "i1", Title: "Movie (1999)", StartMs: 1000, EndMs: 5000,
		Quote: "Never give up", Tags: []string{"motivation", "speech"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || !reflect.DeepEqual(created.Tags, []string{"motivation", "speech"}) {
		t.Fatalf("unexpected clip: %+v", created)
	}

	created.Quote = "Never ever give up"
	created.Tags = nil
	updated, err := s.UpdateClip(t.Context(), created)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Quote != "Never ever give up" || len(updated.Tags) != 0 {
		t.Errorf("update not applied: %+v", updated)
	}

	if err := s.DeleteClip(t.Context(), "u1", created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetClip(t.Context(), "u1", created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestClipsAreIsolatedPerUser(t *testing.T) {
	s := newStore(t)
	c, _ := s.CreateClip(t.Context(), Clip{UserID: "alice", ItemID: "i", Title: "t", EndMs: 1})

	if _, err := s.GetClip(t.Context(), "bob", c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob can read alice's clip: %v", err)
	}
	if err := s.DeleteClip(t.Context(), "bob", c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob can delete alice's clip: %v", err)
	}
	if _, err := s.UpdateClip(t.Context(), Clip{ID: c.ID, UserID: "bob"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob can update alice's clip: %v", err)
	}
	if list, _ := s.ListClips(t.Context(), "bob", ClipFilter{}); len(list) != 0 {
		t.Errorf("bob sees alice's clips: %v", list)
	}
}

func TestListClipsFilters(t *testing.T) {
	s := newStore(t)
	mk := func(title, quote string, tags ...string) {
		if _, err := s.CreateClip(t.Context(), Clip{UserID: "u", ItemID: "i", Title: title, Quote: quote, EndMs: 1, Tags: tags}); err != nil {
			t.Fatal(err)
		}
	}
	mk("Rocky (1976)", "It ain't about how hard you hit", "motivation")
	mk("Show S01E01 - Pilot", "100% sure", "funny", "motivation-ish")

	cases := []struct {
		name   string
		filter ClipFilter
		want   int
	}{
		{"all", ClipFilter{}, 2},
		{"query in quote", ClipFilter{Query: "hard"}, 1},
		{"query in title", ClipFilter{Query: "pilot"}, 1},
		{"percent is literal", ClipFilter{Query: "100%"}, 1},
		{"underscore is literal", ClipFilter{Query: "_"}, 0},
		{"exact tag", ClipFilter{Tag: "motivation"}, 1},
		{"tag is not a prefix match", ClipFilter{Tag: "motiv"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ListClips(t.Context(), "u", tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Errorf("got %d clips, want %d", len(got), tc.want)
			}
		})
	}
}

func TestSessions(t *testing.T) {
	s := newStore(t)
	token, err := s.CreateSession(t.Context(), Session{UserID: "u", UserName: "alice", JellyfinToken: "jf", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSession(t.Context(), token)
	if err != nil || got.UserID != "u" || got.JellyfinToken != "jf" {
		t.Fatalf("unexpected session %+v, err %v", got, err)
	}
	if _, err := s.GetSession(t.Context(), "wrong"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown token, got %v", err)
	}

	if err := s.DeleteSession(t.Context(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession(t.Context(), token); !errors.Is(err, ErrNotFound) {
		t.Errorf("session survived logout: %v", err)
	}
}

func TestExpiredSessionsAreRejectedAndPurged(t *testing.T) {
	s := newStore(t)
	token, _ := s.CreateSession(t.Context(), Session{UserID: "u", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, err := s.GetSession(t.Context(), token); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session accepted: %v", err)
	}
	if err := s.DeleteExpiredSessions(t.Context()); err != nil {
		t.Fatal(err)
	}
}
