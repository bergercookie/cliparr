package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

func logTo(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func clientFor(t *testing.T, handler http.Handler) (*jellyfin.Client, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	jf, err := jellyfin.New(server.URL, "device", "test")
	if err != nil {
		t.Fatal(err)
	}
	return jf, server.URL
}

func TestWatchJellyfinReportsVersionOnceItAnswers(t *testing.T) {
	var calls atomic.Int32
	jf, address := clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 { // Jellyfin is still starting
			http.Error(w, "loading", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"abc","ServerName":"Home","Version":"12.2.0"}`))
	}))

	var out bytes.Buffer
	watchJellyfin(t.Context(), logTo(&out), jf, address, time.Millisecond, false)

	text := out.String()
	if strings.Count(text, "cannot reach Jellyfin yet") != 1 {
		t.Errorf("want exactly one warning while it was down, log:\n%s", text)
	}
	if !strings.Contains(text, "jellyfin reachable") || !strings.Contains(text, "version=12.2.0") || !strings.Contains(text, "name=Home") {
		t.Errorf("want the version and name once it answers, log:\n%s", text)
	}
	if !strings.Contains(text, "hint=") {
		t.Errorf("the warning should say what to check, log:\n%s", text)
	}
}

func TestWatchJellyfinStopsWhenCliparrShutsDown(t *testing.T) {
	jf, address := clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	var out bytes.Buffer
	go func() {
		watchJellyfin(ctx, logTo(&out), jf, address, time.Hour, false)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher kept running after shutdown")
	}
}

func quickConnectProbe(enabled string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/QuickConnect/Enabled" {
			_, _ = w.Write([]byte(enabled))
			return
		}
		_, _ = w.Write([]byte(`{"Id":"abc","ServerName":"Home","Version":"12.2.0"}`))
	})
}

func TestWatchJellyfinWarnsWhenNobodyCouldSignIn(t *testing.T) {
	jf, address := clientFor(t, quickConnectProbe("false"))
	var out bytes.Buffer

	watchJellyfin(t.Context(), logTo(&out), jf, address, time.Millisecond, true)

	if !strings.Contains(out.String(), "nobody can sign in") {
		t.Errorf("want a warning that nobody can sign in, log:\n%s", out.String())
	}
}

func TestWatchJellyfinStaysQuietWhenQuickConnectIsOnOrPasswordsAreAllowed(t *testing.T) {
	for name, tc := range map[string]struct {
		enabled      string
		passwordsOff bool
	}{
		"quick connect on, passwords off": {"true", true},
		"quick connect off, passwords on": {"false", false},
	} {
		t.Run(name, func(t *testing.T) {
			jf, address := clientFor(t, quickConnectProbe(tc.enabled))
			var out bytes.Buffer
			watchJellyfin(t.Context(), logTo(&out), jf, address, time.Millisecond, tc.passwordsOff)
			if strings.Contains(out.String(), "nobody can sign in") {
				t.Errorf("no warning expected, log:\n%s", out.String())
			}
		})
	}
}
