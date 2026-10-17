package api

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

// This file lets a user continue a clip in Jellyfin itself. Jellyfin's web
// addresses cannot carry a start time, so there are two ways to land on the
// exact moment:
//   - play it on a running Jellyfin client (TV, phone, another browser tab),
//     which Jellyfin can be told to start at any position; or
//   - set the user's resume point to the clip start and open the item in
//     Jellyfin's web interface, where "Resume" then starts at that moment.

type deviceJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Client string `json:"client"`
}

type handoffJSON struct {
	// WebURL opens the item in Jellyfin's web interface.
	WebURL string `json:"webUrl"`
	// StartMs is the clip start, which both ways of opening Jellyfin use.
	StartMs int64 `json:"startMs"`
	// ResumeMs is where the user would currently resume, so the app can warn
	// before replacing it.
	ResumeMs int64        `json:"resumeMs"`
	Devices  []deviceJSON `json:"devices"`
}

func (s *Server) clipHandoff(w http.ResponseWriter, r *http.Request, p principal) {
	clip, ok := s.ownedClip(w, r, p)
	if !ok {
		return
	}
	item, err := s.jf.Item(r.Context(), p.JellyfinToken, p.UserID, clip.ItemID)
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}

	out := handoffJSON{
		WebURL:   s.jellyfinWebURL(r, clip.ItemID),
		StartMs:  clip.StartMs,
		ResumeMs: item.ResumeMs,
		Devices:  []deviceJSON{},
	}
	// Having no devices is normal; failing to ask is not worth failing the request for.
	devices, err := s.jf.Devices(r.Context(), p.JellyfinToken, p.UserID)
	if err != nil {
		s.log.Warn("could not list Jellyfin devices", "err", err)
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, deviceJSON{ID: d.ID, Name: d.Name, Client: d.Client})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) clipPlayOnDevice(w http.ResponseWriter, r *http.Request, p principal) {
	var req struct {
		DeviceID string `json:"deviceId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	clip, ok := s.ownedClip(w, r, p)
	if !ok || !s.ensureVisible(w, r, p, clip.ItemID) {
		return
	}

	// An API key could control anyone's device, so only accept devices the user could control.
	devices, err := s.jf.Devices(r.Context(), p.JellyfinToken, p.UserID)
	if err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	known := slices.ContainsFunc(devices, func(d jellyfin.Device) bool { return d.ID == req.DeviceID })
	if !known {
		writeError(w, http.StatusNotFound, "that device is not available; open Jellyfin on it and try again")
		return
	}
	if err := s.jf.PlayOnDevice(r.Context(), p.JellyfinToken, req.DeviceID, clip.ItemID, clip.StartMs); err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clipSetResumePoint makes Jellyfin's Resume button start at the clip. It
// replaces the user's real resume position, so the app asks first.
func (s *Server) clipSetResumePoint(w http.ResponseWriter, r *http.Request, p principal) {
	clip, ok := s.ownedClip(w, r, p)
	if !ok || !s.ensureVisible(w, r, p, clip.ItemID) {
		return
	}
	if err := s.jf.SetResumePoint(r.Context(), p.JellyfinToken, p.UserID, clip.ItemID, clip.StartMs); err != nil {
		s.jellyfinError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// jellyfinWebURL is the address of an item in Jellyfin's web interface as users see it.
func (s *Server) jellyfinWebURL(r *http.Request, itemID string) string {
	link := s.jellyfinPublicURL + "/web/index.html#!/details?id=" + url.QueryEscape(itemID)
	if id := s.serverID(r); id != "" {
		link += "&serverId=" + url.QueryEscape(id)
	}
	return link
}

// serverID is Jellyfin's server id, or "" if it cannot be fetched (links work without it).
func (s *Server) serverID(r *http.Request) string {
	if id, ok := s.serverIDs.get("id"); ok {
		return id
	}
	id, err := s.jf.ServerID(r.Context())
	if err != nil {
		s.log.Warn("could not read the Jellyfin server id", "err", err)
		return ""
	}
	s.serverIDs.set("id", id)
	return id
}
