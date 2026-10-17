// Package fakejellyfin is a tiny in-memory imitation of the Jellyfin API.
//
// It backs the Go tests, the end-to-end script, the screenshot script and
// `just dev`, so Cliparr can be developed without a real Jellyfin installation.
package fakejellyfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// ServerID is the id the fake reports for itself.
const ServerID = "fake-server-id"

// TVDeviceID is the id of the one remote-controllable device the fake offers.
const TVDeviceID = "session-tv"

// users maps username to password. Every password is "pw".
var users = []string{"alice", "bob"}

// PlayCommand is a remote "play this" request received by the fake.
type PlayCommand struct {
	DeviceID   string `json:"deviceId"`
	ItemID     string `json:"itemId"`
	StartTicks int64  `json:"startTicks"`
}

// Server is the fake Jellyfin. It is an http.Handler.
type Server struct {
	mux         *http.ServeMux
	video       []byte
	contentType string
	legacyData  bool

	mu       sync.Mutex
	resume   map[string]int64 // "user/item" -> position in ticks
	lastPlay *PlayCommand
}

// Option customises the fake.
type Option func(*Server)

// WithVideo makes the fake stream the given video file instead of zero bytes.
func WithVideo(path string) Option {
	return func(s *Server) {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("fakejellyfin: cannot read video: %v", err))
		}
		s.video = data
		s.contentType = mime.TypeByExtension(filepath.Ext(path))
	}
}

// WithLegacyUserData makes the fake answer only the pre-10.9 user data address.
func WithLegacyUserData() Option {
	return func(s *Server) { s.legacyData = true }
}

// New returns a fake implementing the Jellyfin endpoints Cliparr uses.
func New(options ...Option) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		video:       make([]byte, 1<<20),
		contentType: "video/mp4",
		resume:      map[string]int64{},
	}
	for _, option := range options {
		option(s)
	}
	s.mux.HandleFunc("GET /System/Info/Public", s.systemInfo)
	s.mux.HandleFunc("POST /Users/AuthenticateByName", s.authenticate)
	s.mux.HandleFunc("GET /Users", s.listUsers)
	s.mux.HandleFunc("GET /Items", s.listItems)
	s.mux.HandleFunc("GET /Videos/{id}/stream", s.stream)
	s.mux.HandleFunc("GET /Items/{id}/Images/Primary", s.image)
	s.mux.HandleFunc("GET /Sessions", s.listSessions)
	s.mux.HandleFunc("POST /Sessions/{id}/Playing", s.play)
	s.mux.HandleFunc("POST /UserItems/{id}/UserData", s.setUserData)
	s.mux.HandleFunc("POST /Users/{userId}/Items/{id}/UserData", s.setUserDataLegacy)
	// Not part of Jellyfin: lets end-to-end tests see what Cliparr asked for.
	s.mux.HandleFunc("GET /_fake/last-play", s.reportLastPlay)
	s.mux.HandleFunc("GET /_fake/resume", s.reportResume)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// LastPlay returns the most recent remote play command.
func (s *Server) LastPlay() (PlayCommand, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastPlay == nil {
		return PlayCommand{}, false
	}
	return *s.lastPlay, true
}

// ResumeTicks returns the stored resume position of an item for a user.
func (s *Server) ResumeTicks(user, itemID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resume[user+"/"+itemID]
}

func (s *Server) systemInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"Id": ServerID, "ServerName": "Fake Jellyfin"})
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	user, ok := requestUser(r)
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
			"UserData":     map[string]int64{"PlaybackPositionTicks": s.ResumeTicks(user, it.ID)},
		})
	}
	writeJSON(w, map[string]any{"Items": out, "TotalRecordCount": len(out)})
}

// stream serves the configured video (zero bytes by default) with Range support.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	s.serveBlob(w, r, s.contentType, s.video)
}

func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	s.serveBlob(w, r, "image/png", []byte("\x89PNG fake"))
}

func (s *Server) serveBlob(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
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

// listSessions offers one controllable TV and one client that cannot be controlled.
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := requestUser(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, []map[string]any{
		{"Id": TVDeviceID, "DeviceName": "Living Room TV", "Client": "Jellyfin Android TV", "SupportsRemoteControl": true},
		{"Id": "session-cliparr", "DeviceName": "Cliparr Server", "Client": "Cliparr", "SupportsRemoteControl": false},
	})
}

func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	if _, ok := requestUser(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	ticks, err := strconv.ParseInt(q.Get("startPositionTicks"), 10, 64)
	if q.Get("playCommand") != "PlayNow" || q.Get("itemIds") == "" || err != nil {
		http.Error(w, "bad play request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.lastPlay = &PlayCommand{DeviceID: r.PathValue("id"), ItemID: q.Get("itemIds"), StartTicks: ticks}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setUserData(w http.ResponseWriter, r *http.Request) {
	if s.legacyData {
		http.NotFound(w, r)
		return
	}
	user, ok := requestUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.storeResume(w, r, user, r.PathValue("id"))
}

func (s *Server) setUserDataLegacy(w http.ResponseWriter, r *http.Request) {
	if _, ok := requestUser(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.storeResume(w, r, strings.TrimPrefix(r.PathValue("userId"), "id-"), r.PathValue("id"))
}

func (s *Server) storeResume(w http.ResponseWriter, r *http.Request, user, itemID string) {
	var body struct{ PlaybackPositionTicks int64 }
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.resume[user+"/"+itemID] = body.PlaybackPositionTicks
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reportLastPlay(w http.ResponseWriter, _ *http.Request) {
	play, ok := s.LastPlay()
	if !ok {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, play)
}

func (s *Server) reportResume(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	writeJSON(w, map[string]int64{"ticks": s.ResumeTicks(q.Get("user"), q.Get("item"))})
}

// requestUser identifies the caller: a user token names the user itself, while
// the API key acts for whichever user the userId parameter names.
func requestUser(r *http.Request) (string, bool) {
	if hasAPIKey(r) {
		q := r.URL.Query()
		id := q.Get("userId")
		if id == "" {
			id = q.Get("controllableByUserId")
		}
		if id == "" {
			// The user data endpoints carry the user in the address instead.
			id = "id-" + strings.TrimPrefix(r.PathValue("userId"), "id-")
		}
		return strings.TrimPrefix(id, "id-"), true
	}
	return userFromToken(r)
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
