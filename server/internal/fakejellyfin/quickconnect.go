package fakejellyfin

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// quickConnectRequest is a sign-in that started on a device and waits for a
// logged-in user to type its code into one of their own Jellyfin apps.
type quickConnectRequest struct {
	code string
	user string // empty until a user approved it
}

// WithoutQuickConnect makes the fake answer like a Jellyfin on which Quick
// Connect is switched off, which is how a Jellyfin server starts out for some versions.
func WithoutQuickConnect() Option {
	return func(s *Server) { s.noQuickConnect = true }
}

func (s *Server) quickConnectEnabled(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, !s.noQuickConnect)
}

func (s *Server) quickConnectInitiate(w http.ResponseWriter, _ *http.Request) {
	if s.noQuickConnect {
		http.Error(w, "Quick connect is not active on this server.", http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quickCount++
	request := &quickConnectRequest{code: fmt.Sprintf("%06d", 100000+s.quickCount)}
	secret := fmt.Sprintf("quick-secret-%d", s.quickCount)
	s.quick[secret] = request
	writeJSON(w, map[string]any{"Authenticated": false, "Secret": secret, "Code": request.code, "AppName": "Cliparr"})
}

func (s *Server) quickConnectState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	secret := r.URL.Query().Get("secret")
	request, ok := s.quick[secret]
	if !ok {
		http.Error(w, "unknown secret", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"Authenticated": request.user != "", "Secret": secret, "Code": request.code})
}

// quickConnectAuthorize is what a user's own Jellyfin app calls after they type the code.
func (s *Server) quickConnectAuthorize(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromToken(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	code := r.URL.Query().Get("code")
	for _, request := range s.quick {
		if request.code == code {
			request.user = user
			writeJSON(w, true)
			return
		}
	}
	http.Error(w, "unknown code", http.StatusNotFound)
}

func (s *Server) authenticateQuickConnect(w http.ResponseWriter, r *http.Request) {
	var req struct{ Secret string }
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	defer s.mu.Unlock()
	request, ok := s.quick[req.Secret]
	if !ok || request.user == "" {
		http.Error(w, "Unknown quick connect secret", http.StatusUnauthorized)
		return
	}
	delete(s.quick, req.Secret) // a secret signs in once
	writeJSON(w, map[string]any{
		"AccessToken": "token-" + request.user,
		"User":        map[string]string{"Id": "id-" + request.user, "Name": request.user},
	})
}
