package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

// This file lets people sign in with Jellyfin's Quick Connect: Cliparr asks Jellyfin
// for a code, the user types it into one of their own Jellyfin apps, and Cliparr
// then receives that user's login. A password never passes through Cliparr.
//
// Jellyfin gives Cliparr a secret next to the code. The secret is what turns an
// approval into a login, so it stays here; the browser only gets an opaque id.

const (
	quickConnectTTL         = 5 * time.Minute // about as long as Jellyfin keeps a code
	maxPendingQuickConnects = 100
)

// pendingQuickConnects remembers the secrets of sign-ins that wait for approval.
type pendingQuickConnects struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	pending map[string]pendingQuickConnect
}

type pendingQuickConnect struct {
	secret  string
	expires time.Time
}

func newPendingQuickConnects(ttl time.Duration, max int) *pendingQuickConnects {
	return &pendingQuickConnects{ttl: ttl, max: max, pending: map[string]pendingQuickConnect{}}
}

// add stores a secret and returns the id the browser polls with. It refuses when
// too many sign-ins wait already, so nobody can fill the memory by asking.
func (p *pendingQuickConnects) add(secret string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for id, entry := range p.pending {
		if now.After(entry.expires) {
			delete(p.pending, id)
		}
	}
	if len(p.pending) >= p.max {
		return "", false
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", false
	}
	id := hex.EncodeToString(raw)
	p.pending[id] = pendingQuickConnect{secret: secret, expires: now.Add(p.ttl)}
	return id, true
}

func (p *pendingQuickConnects) secret(id string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.pending[id]
	if !ok || time.Now().After(entry.expires) {
		delete(p.pending, id)
		return "", false
	}
	return entry.secret, true
}

func (p *pendingQuickConnects) remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.pending, id)
}

// startQuickConnect begins a sign-in and returns the code the user must type into Jellyfin.
func (s *Server) startQuickConnect(w http.ResponseWriter, r *http.Request) {
	request, err := s.jf.StartQuickConnect(r.Context())
	switch {
	case errors.Is(err, jellyfin.ErrUnauthorized):
		writeError(w, http.StatusConflict, "Quick Connect is turned off in Jellyfin. An administrator can turn it on in the Jellyfin dashboard.")
		return
	case err != nil:
		s.log.Error("jellyfin quick connect failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Jellyfin")
		return
	}
	id, ok := s.quickConnects.add(request.Secret)
	if !ok {
		writeError(w, http.StatusTooManyRequests, "too many sign-ins are waiting; try again in a few minutes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "code": request.Code, "expiresInSeconds": int(quickConnectTTL.Seconds()),
	})
}

// pollQuickConnect says whether the user has approved the code yet and, once they
// have, signs them in exactly like a password login would.
func (s *Server) pollQuickConnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	secret, ok := s.quickConnects.secret(id)
	if !ok {
		writeError(w, http.StatusGone, "this sign-in has expired; start again")
		return
	}
	approved, err := s.jf.QuickConnectApproved(r.Context(), secret)
	switch {
	case errors.Is(err, jellyfin.ErrNotFound):
		s.quickConnects.remove(id)
		writeError(w, http.StatusGone, "this sign-in has expired; start again")
		return
	case err != nil:
		s.log.Error("jellyfin quick connect failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Jellyfin")
		return
	case !approved:
		writeJSON(w, http.StatusOK, map[string]any{"status": "pending"})
		return
	}

	jfSession, err := s.jf.AuthenticateQuickConnect(r.Context(), secret)
	s.quickConnects.remove(id) // an approved secret signs in once, whatever happens next
	switch {
	case errors.Is(err, jellyfin.ErrUnauthorized), errors.Is(err, jellyfin.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "Jellyfin did not accept the sign-in; start again")
		return
	case err != nil:
		s.log.Error("jellyfin quick connect failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Jellyfin")
		return
	}
	s.startSession(w, r, jfSession, map[string]any{"status": "approved"})
}

// quickConnectAvailable asks Jellyfin, at most once per cache period, whether codes are on.
func (s *Server) quickConnectAvailable(r *http.Request) bool {
	if on, ok := s.quickConnectOn.get("enabled"); ok {
		return on
	}
	on, err := s.jf.QuickConnectEnabled(r.Context())
	if err != nil {
		return false // not worth a failed page, and not worth remembering
	}
	s.quickConnectOn.set("enabled", on)
	return on
}
