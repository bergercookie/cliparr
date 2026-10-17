// Package api implements Cliparr's HTTP API and serves the web frontend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bergercookie/cliparr/server/internal/config"
	"github.com/bergercookie/cliparr/server/internal/jellyfin"
	"github.com/bergercookie/cliparr/server/internal/store"
)

const (
	sessionCookie = "cliparr_session"
	sessionTTL    = 30 * 24 * time.Hour
	// cacheTTL bounds how long proxy-auth lookups are reused. A user removed
	// from Jellyfin or from a library loses access after at most this long.
	cacheTTL = 5 * time.Minute
)

// Options are the dependencies of the API server.
type Options struct {
	Store     *store.Store
	Jellyfin  *jellyfin.Client
	Version   string
	Log       *slog.Logger
	Web       fs.FS // nil when no frontend is bundled
	ProxyAuth config.ProxyAuth
}

// Server wires the HTTP routes to the store and the Jellyfin client.
type Server struct {
	store   *store.Store
	jf      *jellyfin.Client
	version string
	log     *slog.Logger
	web     fs.FS
	proxy   config.ProxyAuth

	proxyUsers   *ttlCache[jellyfin.User] // username -> Jellyfin user
	visibleItems *ttlCache[struct{}]      // "userID/itemID" the user may see
}

// New returns the root handler.
func New(o Options) http.Handler {
	s := &Server{
		store: o.Store, jf: o.Jellyfin, version: o.Version, log: o.Log, web: o.Web, proxy: o.ProxyAuth,
		proxyUsers:   newTTLCache[jellyfin.User](cacheTTL),
		visibleItems: newTTLCache[struct{}](cacheTTL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/info", s.info)
	mux.HandleFunc("POST /api/auth/login", s.login)

	mux.Handle("POST /api/auth/logout", s.authed(s.logout))
	mux.Handle("GET /api/me", s.authed(s.me))
	mux.Handle("GET /api/items", s.authed(s.searchItems))
	mux.Handle("GET /api/items/{id}/image", s.authed(s.itemImage))
	mux.Handle("GET /api/items/{id}/stream", s.authed(s.itemStream))
	mux.Handle("GET /api/clips", s.authed(s.listClips))
	mux.Handle("POST /api/clips", s.authed(s.createClip))
	mux.Handle("GET /api/clips/{id}", s.authed(s.getClip))
	mux.Handle("PUT /api/clips/{id}", s.authed(s.updateClip))
	mux.Handle("DELETE /api/clips/{id}", s.authed(s.deleteClip))
	mux.Handle("/", s.frontend())
	return s.logRequests(mux)
}

// --- authentication ---------------------------------------------------------

// principal is the authenticated caller of a request.
type principal struct {
	token    string // Cliparr session token; empty for proxy-authenticated requests
	viaProxy bool   // authenticated by a trusted reverse proxy instead of a login
	store.Session
}

type authedHandler func(w http.ResponseWriter, r *http.Request, p principal)

// authed accepts, in this order: the user header of a trusted reverse proxy
// (SSO), "Authorization: Bearer <token>" (native apps) or the session cookie
// (the bundled web frontend).
func (s *Server) authed(next authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok, err := s.proxyPrincipal(r)
		switch {
		case errors.Is(err, errNoSuchUser):
			writeError(w, http.StatusForbidden, "your SSO username does not match any Jellyfin user")
			return
		case err != nil:
			s.jellyfinError(w, r, err)
			return
		case ok:
			next(w, r, p)
			return
		}

		token := sessionToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		sess, err := s.store.GetSession(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "session expired")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next(w, r, principal{token: token, Session: sess})
	})
}

func sessionToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password string }
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	jfSession, err := s.jf.Authenticate(r.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, jellyfin.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "invalid Jellyfin username or password")
		return
	case err != nil:
		s.log.Error("jellyfin login failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Jellyfin")
		return
	}

	expires := time.Now().Add(sessionTTL)
	token, err := s.store.CreateSession(r.Context(), store.Session{
		UserID: jfSession.User.ID, UserName: jfSession.User.Name,
		JellyfinToken: jfSession.Token, ExpiresAt: expires,
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.store.DeleteExpiredSessions(r.Context()); err != nil {
		s.log.Warn("purge expired sessions", "err", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": jfSession.User})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, p principal) {
	if err := s.store.DeleteSession(r.Context(), p.token); err != nil {
		s.internalError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request, p principal) {
	writeJSON(w, http.StatusOK, map[string]any{"id": p.UserID, "name": p.UserName, "viaProxy": p.viaProxy})
}

// --- misc -------------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// info is public so apps can verify a server URL before showing the login form.
func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "cliparr", "version": s.version})
}

func (s *Server) frontend() http.Handler {
	if s.web == nil {
		return http.NotFoundHandler()
	}
	files := http.FileServerFS(s.web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(s.web, "index.html"); err != nil {
			http.Error(w, "Cliparr frontend not bundled in this build (run `just build-web`).", http.StatusNotFound)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// --- helpers ----------------------------------------------------------------

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		// Only the path is logged: query strings may contain search terms.
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController (and the reverse proxy) reach the real writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

// jellyfinError maps a Jellyfin client error to an HTTP response.
func (s *Server) jellyfinError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, jellyfin.ErrNotFound):
		writeError(w, http.StatusNotFound, "item not found")
	case errors.Is(err, jellyfin.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "Jellyfin rejected the session, please log in again")
	case errors.Is(err, context.Canceled):
		// Client went away; nothing to report.
	default:
		s.log.Error("jellyfin request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Jellyfin")
	}
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON reads a JSON body. Requiring the JSON content type also protects
// the cookie-authenticated endpoints from cross-site form posts.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}
