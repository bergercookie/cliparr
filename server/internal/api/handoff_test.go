package api_test

import (
	"strconv"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
)

const publicURL = "https://jellyfin.example.com"

func withPublicURL(o *api.Options) { o.JellyfinPublicURL = publicURL }

type handoff struct {
	WebURL   string `json:"webUrl"`
	StartMs  int64  `json:"startMs"`
	ResumeMs int64  `json:"resumeMs"`
	Devices  []struct{ ID, Name, Client string }
}

func clipPath(id int64, suffix string) string {
	return "/api/clips/" + strconv.FormatInt(id, 10) + suffix
}

func TestHandoffDescribesHowToOpenTheClipInJellyfin(t *testing.T) {
	e := newEnv(t, withPublicURL)
	token := e.login("alice")
	id := e.createClip(token, 80_000, 90_000)

	var h handoff
	if code := e.call("GET", clipPath(id, "/jellyfin"), token, nil, &h); code != 200 {
		t.Fatalf("status %d", code)
	}
	if want := publicURL + "/web/index.html#!/details?id=movie-rocky&serverId=" + fakejellyfin.ServerID; h.WebURL != want {
		t.Errorf("web URL:\n got %s\nwant %s", h.WebURL, want)
	}
	if h.StartMs != 80_000 || h.ResumeMs != 0 {
		t.Errorf("start %d, resume %d", h.StartMs, h.ResumeMs)
	}
	if len(h.Devices) != 1 || h.Devices[0].ID != fakejellyfin.TVDeviceID || h.Devices[0].Name != "Living Room TV" {
		t.Errorf("devices: %+v (clients that cannot be remote-controlled must be left out)", h.Devices)
	}
}

func TestPlayOnDeviceStartsAtTheExactTime(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")
	id := e.createClip(token, 80_000, 90_000)

	code := e.call("POST", clipPath(id, "/jellyfin/play"), token, map[string]string{"deviceId": fakejellyfin.TVDeviceID}, nil)
	if code != 204 {
		t.Fatalf("status %d", code)
	}
	play, ok := e.fake.LastPlay()
	want := fakejellyfin.PlayCommand{DeviceID: fakejellyfin.TVDeviceID, ItemID: "movie-rocky", StartTicks: 80_000 * 10_000}
	if !ok || play != want {
		t.Errorf("Jellyfin was told %+v, want %+v", play, want)
	}
}

func TestPlayOnDeviceRefusesDevicesTheUserCannotControl(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")
	id := e.createClip(token, 1000, 2000)

	for _, device := range []string{"someone-elses-tv", "session-cliparr", ""} {
		if code := e.call("POST", clipPath(id, "/jellyfin/play"), token, map[string]string{"deviceId": device}, nil); code != 404 {
			t.Errorf("device %q: status %d, want 404", device, code)
		}
	}
	if _, ok := e.fake.LastPlay(); ok {
		t.Error("Jellyfin was told to play on an unavailable device")
	}
}

func TestSetResumePointMakesResumeStartAtTheClip(t *testing.T) {
	e := newEnv(t)
	token := e.login("alice")
	id := e.createClip(token, 80_000, 90_000)

	if code := e.call("POST", clipPath(id, "/jellyfin/resume"), token, nil, nil); code != 204 {
		t.Fatalf("status %d", code)
	}
	if got := e.fake.ResumeTicks("alice", "movie-rocky"); got != 80_000*10_000 {
		t.Errorf("resume point is %d ticks", got)
	}
	var h handoff
	e.call("GET", clipPath(id, "/jellyfin"), token, nil, &h)
	if h.ResumeMs != 80_000 {
		t.Errorf("the handoff reports resume %d ms, want 80000", h.ResumeMs)
	}
}

func TestSetResumePointWorksWithOlderJellyfin(t *testing.T) {
	e := newEnvWithFake(t, fakejellyfin.New(fakejellyfin.WithLegacyUserData()))
	token := e.login("alice")
	id := e.createClip(token, 5000, 6000)

	if code := e.call("POST", clipPath(id, "/jellyfin/resume"), token, nil, nil); code != 204 {
		t.Fatalf("status %d", code)
	}
	if got := e.fake.ResumeTicks("alice", "movie-rocky"); got != 5000*10_000 {
		t.Errorf("resume point is %d ticks", got)
	}
}

func TestHandoffBelongsToTheClipOwner(t *testing.T) {
	e := newEnv(t)
	id := e.createClip(e.login("alice"), 1000, 2000)
	bob := e.login("bob")

	if code := e.call("GET", clipPath(id, "/jellyfin"), bob, nil, nil); code != 404 {
		t.Errorf("handoff: status %d, want 404", code)
	}
	if code := e.call("POST", clipPath(id, "/jellyfin/play"), bob, map[string]string{"deviceId": fakejellyfin.TVDeviceID}, nil); code != 404 {
		t.Errorf("play: status %d, want 404", code)
	}
	if code := e.call("POST", clipPath(id, "/jellyfin/resume"), bob, nil, nil); code != 404 {
		t.Errorf("resume: status %d, want 404", code)
	}
	if _, ok := e.fake.LastPlay(); ok || e.fake.ResumeTicks("alice", "movie-rocky") != 0 {
		t.Error("bob changed something in Jellyfin through alice's clip")
	}
}

func TestHandoffWorksForSSOUsers(t *testing.T) {
	e := newEnv(t, withProxyAuth("127.0.0.0/8"), withPublicURL)
	id := e.createClip(e.login("alice"), 7000, 8000)

	// alice, now identified by the proxy header and acting through the API key.
	var h handoff
	if code := e.asUser(clipPath(id, "/jellyfin"), "alice", &h); code != 200 || len(h.Devices) != 1 {
		t.Fatalf("handoff via proxy: status %d, %+v", code, h)
	}
}
