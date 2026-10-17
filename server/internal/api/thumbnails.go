package api

import (
	"net/http"

	"github.com/bergercookie/cliparr/server/internal/thumbnail"
)

// clipThumbnail serves a preview picture of the clip: a frame from its middle.
// Without ffmpeg, or if making the picture fails, it falls back to the movie's
// poster so cards still show something.
func (s *Server) clipThumbnail(w http.ResponseWriter, r *http.Request, p principal) {
	clip, ok := s.ownedClip(w, r, p)
	if !ok || !s.ensureVisible(w, r, p, clip.ItemID) {
		return
	}

	if s.thumbs != nil && s.thumbs.Enabled() {
		path := s.thumbs.Path(clip.ID, clip.StartMs, clip.EndMs)
		url, auth := s.jf.StreamSource(p.JellyfinToken, clip.ItemID)
		middle := clip.StartMs + (clip.EndMs-clip.StartMs)/2
		err := s.thumbs.Ensure(r.Context(), path, thumbnail.Source{URL: url, AuthHeader: auth}, middle)
		if err == nil {
			w.Header().Set("Cache-Control", "private, max-age=86400")
			http.ServeFile(w, r, path)
			return
		}
		s.log.Warn("thumbnail failed, using the poster", "clip", clip.ID, "err", err)
	}

	w.Header().Set("X-Cliparr-Thumbnail", "poster")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	s.jf.ProxyImage(w, r, p.JellyfinToken, clip.ItemID)
}

// removeThumbnails drops the stored pictures of a clip that changed or was deleted.
func (s *Server) removeThumbnails(clipID int64) {
	if s.thumbs != nil {
		s.thumbs.Remove(clipID)
	}
}
