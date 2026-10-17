package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/bergercookie/cliparr/server/internal/store"
)

// The browser extension runs inside Jellyfin's web client, which already holds
// the user's Jellyfin token. Instead of asking for a second login, the
// extension sends that token as "Authorization: Jellyfin <token>" and Cliparr
// asks Jellyfin who it belongs to. Nothing is stored: every request is checked
// (with a short cache), and from then on the token is used for Jellyfin calls
// exactly like after a password login, so Jellyfin still decides what the user
// may see.

// jellyfinTokenFrom returns the token of an "Authorization: Jellyfin <token>" header.
func jellyfinTokenFrom(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Jellyfin ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

// tokenPrincipal checks a Jellyfin token and returns the user it belongs to.
func (s *Server) tokenPrincipal(ctx context.Context, token string) (principal, error) {
	sum := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(sum[:])

	user, ok := s.tokenUsers.get(key)
	if !ok {
		var err error
		if user, err = s.jf.UserByToken(ctx, token); err != nil {
			return principal{}, err
		}
		s.tokenUsers.set(key, user)
	}
	return principal{Session: store.Session{UserID: user.ID, UserName: user.Name, JellyfinToken: token}}, nil
}
