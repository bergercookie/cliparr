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
	"net/url"
	"os"
	"path/filepath"
	"slices"
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

	// The codec of the file's audio, as Jellyfin reports it when asked for
	// MediaSources. Empty reads as AAC.
	AudioCodec string
}

var catalog = []item{
	{ID: "movie-rocky", Name: "Rocky", Type: "Movie", Year: 1976, Minutes: 120},
	{ID: "episode-pilot", Name: "Pilot", Type: "Episode", SeriesName: "Fake Show", Season: 1, Episode: 1, Minutes: 45},
	{ID: "movie-dolby", Name: "Dolby Dreams", Type: "Movie", Year: 2021, Minutes: 100, AudioCodec: "ac3"},
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

	noQuickConnect bool
	quickCount     int
	quick          map[string]*quickConnectRequest // secret -> pending sign-in

	mu         sync.Mutex
	resume     map[string]int64 // "user/item" -> position in ticks
	lastPlay   *PlayCommand
	audioAsked []url.Values        // queries of the audio streams asked for
	playing    map[string]progress // device id -> what that client last reported playing
}

// progress is what a Jellyfin client reports while it plays something.
type progress struct {
	ItemID        string
	PositionTicks int64
	IsPaused      bool
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
		playing:     map[string]progress{},
		quick:       map[string]*quickConnectRequest{},
	}
	for _, option := range options {
		option(s)
	}
	s.mux.HandleFunc("GET /System/Info/Public", s.systemInfo)
	s.mux.HandleFunc("POST /Users/AuthenticateByName", s.authenticate)
	s.mux.HandleFunc("GET /QuickConnect/Enabled", s.quickConnectEnabled)
	s.mux.HandleFunc("POST /QuickConnect/Initiate", s.quickConnectInitiate)
	s.mux.HandleFunc("GET /QuickConnect/Connect", s.quickConnectState)
	s.mux.HandleFunc("POST /QuickConnect/Authorize", s.quickConnectAuthorize)
	s.mux.HandleFunc("POST /Users/AuthenticateWithQuickConnect", s.authenticateQuickConnect)
	s.mux.HandleFunc("GET /Users", s.listUsers)
	s.mux.HandleFunc("GET /Users/Me", s.me)
	s.mux.HandleFunc("GET /Items", s.listItems)
	s.mux.HandleFunc("GET /Videos/{id}/stream", s.stream)
	s.mux.HandleFunc("GET /Audio/{id}/stream.mp3", s.audioStream)
	s.mux.HandleFunc("GET /Items/{id}/Images/Primary", s.image)
	s.mux.HandleFunc("GET /Sessions", s.listSessions)
	s.mux.HandleFunc("POST /Sessions/{id}/Playing", s.play)
	s.mux.HandleFunc("POST /Sessions/Playing/Progress", s.reportProgress)
	s.mux.HandleFunc("GET /web/index.html", s.webPlayer)
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
	writeJSON(w, map[string]string{"Id": ServerID, "ServerName": "Fake Jellyfin", "Version": "10.11.0-fake"})
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

// me answers like Jellyfin: the user of a user token, 400 for an API key
// (which has no user) and 401 for anything else.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if user, ok := userFromToken(r); ok {
		writeJSON(w, map[string]string{"Id": "id-" + user, "Name": user})
		return
	}
	if hasAPIKey(r) {
		http.Error(w, "no user", http.StatusBadRequest)
		return
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		entry := map[string]any{
			"Id": it.ID, "Name": it.Name, "Type": it.Type, "SeriesName": it.SeriesName,
			"ParentIndexNumber": it.Season, "IndexNumber": it.Episode, "ProductionYear": it.Year,
			"RunTimeTicks": int64(it.Minutes) * ticksPerMinute,
			"UserData":     map[string]int64{"PlaybackPositionTicks": s.ResumeTicks(user, it.ID)},
		}
		if strings.Contains(q.Get("fields"), "MediaSources") {
			entry["MediaSources"] = []any{mediaSource(it)}
		}
		out = append(out, entry)
	}
	writeJSON(w, map[string]any{"Items": out, "TotalRecordCount": len(out)})
}

// mediaSource describes an item's file the way Jellyfin's MediaSources do.
func mediaSource(it item) map[string]any {
	audio := it.AudioCodec
	if audio == "" {
		audio = "aac"
	}
	return map[string]any{
		"Container": "mp4",
		"MediaStreams": []map[string]any{
			{"Type": "Video", "Codec": "h264"},
			{"Type": "Audio", "Codec": audio, "IsDefault": true},
		},
	}
}

// audioStream stands in for Jellyfin's audio transcode: it notes what was
// asked for and serves the configured video's bytes, never as partial content.
func (s *Server) audioStream(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.audioAsked = append(s.audioAsked, r.URL.Query())
	s.mu.Unlock()
	r.Header.Del("Range")
	s.serveBlob(w, r, "audio/mpeg", s.video)
}

// AudioStreams returns the queries of the audio streams asked for so far.
func (s *Server) AudioStreams() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.audioAsked)
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

// listSessions offers one controllable TV and one client that cannot be
// controlled. With ?deviceId=X it instead returns that client's session, if it
// is playing something, like Jellyfin does.
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := requestUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if device := r.URL.Query().Get("deviceId"); device != "" {
		s.mu.Lock()
		p, playing := s.playing[device]
		s.mu.Unlock()
		sessions := []map[string]any{}
		if playing {
			sessions = append(sessions, map[string]any{
				"Id": "session-" + device, "DeviceId": device, "DeviceName": "Fake Browser", "Client": "Jellyfin Web",
				"UserId": "id-" + user, "SupportsRemoteControl": true,
				"NowPlayingItem": map[string]string{"Id": p.ItemID},
				"PlayState":      map[string]any{"PositionTicks": p.PositionTicks, "IsPaused": p.IsPaused},
			})
		}
		writeJSON(w, sessions)
		return
	}
	writeJSON(w, []map[string]any{
		{"Id": TVDeviceID, "DeviceName": "Living Room TV", "Client": "Jellyfin Android TV", "SupportsRemoteControl": true},
		{"Id": "session-cliparr", "DeviceName": "Cliparr Server", "Client": "Cliparr", "SupportsRemoteControl": false},
	})
}

// reportProgress stores what a client says it is playing, as jellyfin-web does every few seconds.
func (s *Server) reportProgress(w http.ResponseWriter, r *http.Request) {
	if _, ok := requestUser(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body progress
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.ItemID == "" {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	device := headerValue(r.Header.Get("Authorization"), "DeviceId")
	s.mu.Lock()
	s.playing[device] = body
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// headerValue reads key="value" out of a MediaBrowser Authorization header.
func headerValue(header, key string) string {
	_, after, found := strings.Cut(header, key+`="`)
	if !found {
		return ""
	}
	value, _, _ := strings.Cut(after, `"`)
	return value
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

// webPlayer is a minimal stand-in for Jellyfin's web client, for testing the
// browser extension: it keeps its credentials in localStorage like jellyfin-web,
// plays a video, and reports progress. ?user=bob and ?item=ID pick who watches what.
func (s *Server) webPlayer(w http.ResponseWriter, r *http.Request) {
	user, item := r.URL.Query().Get("user"), r.URL.Query().Get("item")
	if user == "" {
		user = "alice"
	}
	if item == "" {
		item = "movie-rocky"
	}
	if !isUser(user) || !isItem(item) {
		http.Error(w, "unknown user or item", http.StatusBadRequest)
		return
	}
	page := strings.NewReplacer("__USER__", user, "__ITEM__", item).Replace(webPlayerPage)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

func isItem(id string) bool {
	for _, it := range catalog {
		if it.ID == id {
			return true
		}
	}
	return false
}

const webPlayerPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Fake Jellyfin</title></head>
<body style="margin:0;background:#101010">
<video id="player" src="/Videos/__ITEM__/stream?static=true&api_key=token-__USER__" controls muted loop
  style="width:100%;max-height:90vh"></video>
<script>
  // What jellyfin-web keeps in localStorage after login.
  localStorage.setItem('jellyfin_credentials', JSON.stringify({Servers: [{
    Id: 'fake-server-id', Name: 'Fake Jellyfin', ManualAddress: location.origin, LocalAddress: location.origin,
    UserId: 'id-__USER__', AccessToken: 'token-__USER__', DateLastAccessed: Date.now()}]}));
  localStorage.setItem('_deviceId2', 'fake-device-1');
  const video = document.getElementById('player');
  // Like jellyfin-web, report what is playing every moment.
  setInterval(() => fetch('/Sessions/Playing/Progress', {
    method: 'POST',
    headers: {'Content-Type': 'application/json',
      'Authorization': 'MediaBrowser Client="Jellyfin Web", Device="Fake", DeviceId="fake-device-1", Version="10.9", Token="token-__USER__"'},
    body: JSON.stringify({ItemId: '__ITEM__', PositionTicks: Math.round(video.currentTime * 10000000), IsPaused: video.paused}),
  }), 500);
</script>
</body></html>`

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

// userFromToken extracts the user from `Token="token-<user>"` in the
// Authorization header, the X-Emby-Token header, or the api_key query
// parameter that browsers use for video elements.
func userFromToken(r *http.Request) (string, bool) {
	for _, key := range []string{r.URL.Query().Get("api_key"), r.Header.Get("X-Emby-Token")} {
		if user, ok := strings.CutPrefix(key, "token-"); ok {
			return user, isUser(user)
		}
	}
	h := r.Header.Get("Authorization")
	i := strings.Index(h, `Token="token-`)
	if i < 0 {
		return "", false
	}
	user, _, _ := strings.Cut(h[i+len(`Token="token-`):], `"`)
	return user, isUser(user)
}

func hasAPIKey(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Authorization"), `Token="`+APIKey+`"`) ||
		r.Header.Get("X-Emby-Token") == APIKey || r.URL.Query().Get("api_key") == APIKey
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
