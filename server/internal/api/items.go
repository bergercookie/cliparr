package api

import (
	"net/http"
	"strings"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

const searchLimit = 25

type itemJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	RuntimeMs int64  `json:"runtimeMs"`
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
	if s.ensureVisible(w, r, p, id) {
		s.jf.ProxyStream(w, r, p.JellyfinToken, id)
	}
}

func toItemJSON(it jellyfin.Item) itemJSON {
	return itemJSON{ID: it.ID, Title: it.Title, Type: it.Type, RuntimeMs: it.RuntimeMs}
}
