package api

import (
	"errors"
	"net/http"
	"net/netip"
	"strings"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

// This file implements single sign-on through a reverse proxy (Authelia,
// Authentik, oauth2-proxy, ...). The proxy authenticates the user and passes
// the username in a header. Cliparr only believes that header when the request
// comes from a configured proxy address, so nobody can fake it by talking to
// Cliparr directly.
//
// SSO users often have no Jellyfin password, so Cliparr uses an administrator
// API key to act for them. An API key is not limited to one user's library,
// which is why every item a proxy-authenticated user streams is first checked
// against that user's own view (see ensureVisible).

var errNoSuchUser = errors.New("no Jellyfin user with that name")

// proxyPrincipal returns the user named by the trusted proxy header.
// ok is false when the request does not carry a usable header.
func (s *Server) proxyPrincipal(r *http.Request) (p principal, ok bool, err error) {
	if !s.proxy.Enabled() {
		return principal{}, false, nil
	}
	name := strings.TrimSpace(r.Header.Get(s.proxy.UserHeader))
	if name == "" {
		return principal{}, false, nil
	}
	if !s.fromTrustedProxy(r) {
		s.log.Warn("ignoring proxy auth header sent from an untrusted address", "remote", r.RemoteAddr)
		return principal{}, false, nil
	}

	key := strings.ToLower(name)
	user, cached := s.proxyUsers.get(key)
	if !cached {
		user, err = s.jf.UserByName(r.Context(), s.proxy.JellyfinAPIKey, name)
		if errors.Is(err, jellyfin.ErrNotFound) {
			return principal{}, false, errNoSuchUser
		}
		if err != nil {
			return principal{}, false, err
		}
		s.proxyUsers.set(key, user)
	}

	p = principal{viaProxy: true}
	p.UserID, p.UserName, p.JellyfinToken = user.ID, user.Name, s.proxy.JellyfinAPIKey
	return p, true, nil
}

func (s *Server) fromTrustedProxy(r *http.Request) bool {
	addr, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && s.proxy.Trusts(addr.Addr())
}

// ensureVisible checks that a proxy-authenticated user may see the item and
// writes an error response if not. Users logged in with their own Jellyfin
// token need no check: Jellyfin enforces their permissions itself.
func (s *Server) ensureVisible(w http.ResponseWriter, r *http.Request, p principal, itemID string) bool {
	if !p.viaProxy {
		return true
	}
	key := p.UserID + "/" + itemID
	if _, ok := s.visibleItems.get(key); ok {
		return true
	}
	if _, err := s.jf.Item(r.Context(), p.JellyfinToken, p.UserID, itemID); err != nil {
		s.jellyfinError(w, r, err)
		return false
	}
	s.visibleItems.set(key, struct{}{})
	return true
}
