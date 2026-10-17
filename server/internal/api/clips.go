package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bergercookie/cliparr/server/internal/store"
)

const (
	maxTextLen = 2000
	maxTags    = 20
	maxTagLen  = 40
)

// clipRequest is the body of create and update requests. ItemID is only used
// when creating; a clip cannot be moved to another item.
type clipRequest struct {
	ItemID  string   `json:"itemId"`
	StartMs int64    `json:"startMs"`
	EndMs   int64    `json:"endMs"`
	Quote   string   `json:"quote"`
	Note    string   `json:"note"`
	Tags    []string `json:"tags"`
}

func (s *Server) listClips(w http.ResponseWriter, r *http.Request, p principal) {
	q := r.URL.Query()
	clips, err := s.store.ListClips(r.Context(), p.UserID, store.ClipFilter{
		Query: strings.TrimSpace(q.Get("q")),
		Tag:   normalizeTag(q.Get("tag")),
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clips)
}

func (s *Server) getClip(w http.ResponseWriter, r *http.Request, p principal) {
	if clip, ok := s.ownedClip(w, r, p); ok {
		writeJSON(w, http.StatusOK, clip)
	}
}

// ownedClip loads the clip named in the address if it belongs to the caller,
// and writes an error response otherwise.
func (s *Server) ownedClip(w http.ResponseWriter, r *http.Request, p principal) (store.Clip, bool) {
	id, ok := clipID(w, r)
	if !ok {
		return store.Clip{}, false
	}
	clip, err := s.store.GetClip(r.Context(), p.UserID, id)
	if s.clipError(w, r, err) {
		return store.Clip{}, false
	}
	return clip, true
}

func (s *Server) createClip(w http.ResponseWriter, r *http.Request, p principal) {
	var req clipRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ItemID == "" {
		writeError(w, http.StatusBadRequest, "itemId is required")
		return
	}
	// Fetching the item with the user's own token proves they may access it
	// and gives us the authoritative title and runtime.
	item, err := s.jf.Item(r.Context(), p.JellyfinToken, p.UserID, req.ItemID)
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	clip, msg := req.toClip(p.UserID, item.RuntimeMs)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	clip.ItemID, clip.Title = item.ID, item.Title

	created, err := s.store.CreateClip(r.Context(), clip)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateClip(w http.ResponseWriter, r *http.Request, p principal) {
	id, ok := clipID(w, r)
	if !ok {
		return
	}
	var req clipRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	existing, err := s.store.GetClip(r.Context(), p.UserID, id)
	if s.clipError(w, r, err) {
		return
	}
	item, err := s.jf.Item(r.Context(), p.JellyfinToken, p.UserID, existing.ItemID)
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	clip, msg := req.toClip(p.UserID, item.RuntimeMs)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	clip.ID = id
	updated, err := s.store.UpdateClip(r.Context(), clip)
	if s.clipError(w, r, err) {
		return
	}
	s.removeThumbnails(id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteClip(w http.ResponseWriter, r *http.Request, p principal) {
	id, ok := clipID(w, r)
	if !ok {
		return
	}
	if s.clipError(w, r, s.store.DeleteClip(r.Context(), p.UserID, id)) {
		return
	}
	s.removeThumbnails(id)
	w.WriteHeader(http.StatusNoContent)
}

// toClip validates the request against the item runtime. It returns a
// user-facing message when the request is invalid.
func (req clipRequest) toClip(userID string, runtimeMs int64) (store.Clip, string) {
	switch {
	case req.StartMs < 0:
		return store.Clip{}, "startMs must not be negative"
	case req.EndMs <= req.StartMs:
		return store.Clip{}, "endMs must be greater than startMs"
	case runtimeMs > 0 && req.EndMs > runtimeMs:
		return store.Clip{}, "endMs is past the end of the video"
	case len(req.Quote) > maxTextLen || len(req.Note) > maxTextLen:
		return store.Clip{}, "quote and note must be at most 2000 characters"
	}
	tags, msg := cleanTags(req.Tags)
	if msg != "" {
		return store.Clip{}, msg
	}
	return store.Clip{
		UserID: userID, StartMs: req.StartMs, EndMs: req.EndMs,
		Quote: strings.TrimSpace(req.Quote), Note: strings.TrimSpace(req.Note), Tags: tags,
	}, ""
}

func cleanTags(in []string) ([]string, string) {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range in {
		tag := normalizeTag(raw)
		switch {
		case tag == "":
			continue
		case len(tag) > maxTagLen:
			return nil, "tags must be at most 40 characters"
		case strings.Contains(tag, ","):
			return nil, "tags must not contain commas"
		case seen[tag]:
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) > maxTags {
		return nil, "at most 20 tags are allowed"
	}
	return out, ""
}

func normalizeTag(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }

func clipID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid clip id")
		return 0, false
	}
	return id, true
}

// clipError writes a response and returns true if err is non-nil.
func (s *Server) clipError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "clip not found")
	default:
		s.internalError(w, r, err)
	}
	return true
}
