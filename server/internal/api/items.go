package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

const searchLimit = 25

type itemJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	RuntimeMs int64  `json:"runtimeMs"`

	// AudioPlayable is false when browsers cannot decode the file's audio
	// (Dolby Digital, DTS and the like). Fetch the sound from the audio
	// endpoint then. Only set when asking for a single item.
	AudioPlayable *bool `json:"audioPlayable,omitempty"`
}

func (s *Server) searchItems(w http.ResponseWriter, r *http.Request, p principal) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, []itemJSON{})
		return
	}
	items, err := s.jf.Search(r.Context(), p.JellyfinToken, p.UserID, query, searchLimit)
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	out := make([]itemJSON, 0, len(items))
	for _, it := range items {
		out = append(out, toItemJSON(it))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request, p principal) {
	it, err := s.jf.Item(r.Context(), p.JellyfinToken, p.UserID, r.PathValue("id"))
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	out := toItemJSON(it)
	playable := audioPlayable(it)
	out.AudioPlayable = &playable
	writeJSON(w, http.StatusOK, out)
}

// browserAudio lists the audio codecs that every current browser decodes.
var browserAudio = []string{"aac", "mp3", "opus", "vorbis", "flac"}

// audioPlayable reports whether a browser can decode the item's audio. An
// unknown codec counts as playable: guessing wrong should not put a transcode
// in front of files that work.
func audioPlayable(it jellyfin.Item) bool {
	return it.AudioCodec == "" || slices.Contains(browserAudio, it.AudioCodec)
}

func (s *Server) itemImage(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("id")
	if s.ensureVisible(w, r, p, id) {
		s.jf.ProxyImage(w, r, p.JellyfinToken, id)
	}
}

// itemStream serves the video through Cliparr so that the Jellyfin URL and the
// user's Jellyfin token never reach the client.
func (s *Server) itemStream(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("id")
	if !s.ensureVisible(w, r, p, id) {
		return
	}
	s.jf.ProxyStream(w, r, p.JellyfinToken, id)
}

// itemAudio serves the sound of the item as MP3 from startMs on.
func (s *Server) itemAudio(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("id")
	if !s.ensureVisible(w, r, p, id) {
		return
	}
	startMs, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	if r.URL.Query().Get("start") != "" && (err != nil || startMs < 0) {
		writeError(w, http.StatusBadRequest, "start must be a number of milliseconds")
		return
	}
	s.jf.ProxyAudio(w, r, p.JellyfinToken, id, startMs)
}

func toItemJSON(it jellyfin.Item) itemJSON {
	return itemJSON{ID: it.ID, Title: it.Title, Type: it.Type, RuntimeMs: it.RuntimeMs}
}
