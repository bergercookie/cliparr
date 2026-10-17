// Package fakejellyfin is a tiny in-memory imitation of the Jellyfin API.
//
// It backs the Go tests, the end-to-end script and `just dev`, so Cliparr can
// be developed without a real Jellyfin installation.
package fakejellyfin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const ticksPerMinute = 600_000_000

type item struct {
	ID, Name, Type, SeriesName string
	Season, Episode, Year      int
	Minutes                    int
	Only                       string // if set, only this user may see the item
}

var catalog = []item{
	{ID: "movie-rocky", Name: "Rocky", Type: "Movie", Year: 1976, Minutes: 120},
	{ID: "episode-pilot", Name: "Pilot", Type: "Episode", SeriesName: "Fake Show", Season: 1, Episode: 1, Minutes: 45},
	{ID: "movie-secret", Name: "Secret Movie", Type: "Movie", Year: 2001, Minutes: 90, Only: "alice"},
}

// APIKey is the administrator API key accepted by the fake.
const APIKey = "test-api-key"

// users maps username to password. Every password is "pw".
var users = []string{"alice", "bob"}

// New returns a handler implementing the Jellyfin endpoints Cliparr uses.
func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", authenticate)
	mux.HandleFunc("GET /Users", listUsers)
	mux.HandleFunc("GET /Items", listItems)
	mux.HandleFunc("GET /Videos/{id}/stream", stream)
	mux.HandleFunc("GET /Items/{id}/Images/Primary", image)
	return mux
}

func authenticate(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Pw string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Pw != "pw" || !isUser(req.Username) {
		http.Error(w, "bad credentials", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]any{
		"AccessToken": "token-" + req.Username,
		"User":        map[string]string{"Id": "id-" + req.Username, "Name": req.Username},
	})
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	if !hasAPIKey(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out := []map[string]string{}
	for _, u := range users {
		out = append(out, map[string]string{"Id": "id-" + u, "Name": u})
	}
	writeJSON(w, out)
}

func listItems(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromToken(r)
	if hasAPIKey(r) {
		// Like the real server: an API key acts for the user named by userId.
		user, ok = strings.TrimPrefix(r.URL.Query().Get("userId"), "id-"), true
	}
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	search, ids := strings.ToLower(q.Get("searchTerm")), q.Get("ids")
	out := []map[string]any{}
	for _, it := range catalog {
		if !visibleTo(it, user) ||
			(search != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.SeriesName), search)) ||
			(ids != "" && ids != it.ID) {
			continue
		}
		out = append(out, map[string]any{
			"Id": it.ID, "Name": it.Name, "Type": it.Type, "SeriesName": it.SeriesName,
			"ParentIndexNumber": it.Season, "IndexNumber": it.Episode, "ProductionYear": it.Year,
			"RunTimeTicks": int64(it.Minutes) * ticksPerMinute,
		})
	}
	writeJSON(w, map[string]any{"Items": out, "TotalRecordCount": len(out)})
}

// stream serves 1 MiB of zeros with Range support, enough to test proxying.
func stream(w http.ResponseWriter, r *http.Request) {
	serveBlob(w, r, "video/mp4", make([]byte, 1<<20))
}

func image(w http.ResponseWriter, r *http.Request) {
	serveBlob(w, r, "image/png", []byte("\x89PNG fake"))
}

func serveBlob(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	user, ok := userFromToken(r)
	if !ok && !hasAPIKey(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	for _, it := range catalog {
		// An API key is not tied to a user, so it can fetch every item.
		if it.ID == r.PathValue("id") && (hasAPIKey(r) || visibleTo(it, user)) {
			w.Header().Set("Content-Type", contentType)
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
			return
		}
	}
	http.NotFound(w, r)
}

// userFromToken extracts the user from `Token="token-<user>"` in the Authorization header.
func userFromToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	i := strings.Index(h, `Token="token-`)
	if i < 0 {
		return "", false
	}
	user, _, _ := strings.Cut(h[i+len(`Token="token-`):], `"`)
	return user, isUser(user)
}

func hasAPIKey(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Authorization"), `Token="`+APIKey+`"`)
}

func isUser(name string) bool {
	for _, u := range users {
		if u == name {
			return true
		}
	}
	return false
}

func visibleTo(it item, user string) bool { return it.Only == "" || it.Only == user }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
