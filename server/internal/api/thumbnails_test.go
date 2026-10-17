package api_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
	"github.com/bergercookie/cliparr/server/internal/testutil"
	"github.com/bergercookie/cliparr/server/internal/thumbnail"
)

// withThumbnails stores thumbnails in dir, made by the given ffmpeg setting.
func withThumbnails(t *testing.T, ffmpeg, dir string) func(*api.Options) {
	t.Helper()
	maker, err := thumbnail.New(ffmpeg, dir)
	if err != nil {
		t.Fatal(err)
	}
	return func(o *api.Options) { o.Thumbnails = maker }
}

// get fetches a path with a bearer token and returns status, headers and body.
func (e *testEnv) get(path, token string) (int, http.Header, []byte) {
	e.t.Helper()
	req, _ := http.NewRequest("GET", e.url+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, body
}

func (e *testEnv) createClip(token string, startMs, endMs int64) int64 {
	e.t.Helper()
	var clip struct{ ID int64 }
	code := e.call("POST", "/api/clips", token, map[string]any{"itemId": "movie-rocky", "startMs": startMs, "endMs": endMs}, &clip)
	if code != 201 {
		e.t.Fatalf("create clip: status %d", code)
	}
	return clip.ID
}

func TestThumbnailIsAFrameOfTheClip(t *testing.T) {
	dir := t.TempDir()
	e := newEnvWithFake(t, fakejellyfin.New(fakejellyfin.WithVideo(testutil.MakeVideo(t))), withThumbnails(t, "", dir))
	token := e.login("alice")
	id := e.createClip(token, 2000, 6000)

	code, header, body := e.get("/api/clips/"+strconv.FormatInt(id, 10)+"/thumbnail", token)
	if code != 200 || !bytes.HasPrefix(body, []byte{0xFF, 0xD8}) {
		t.Fatalf("status %d, body starts % x", code, body[:min(4, len(body))])
	}
	if header.Get("X-Cliparr-Thumbnail") != "" {
		t.Error("got the poster fallback although ffmpeg works")
	}
	if _, err := os.Stat(filepath.Join(dir, strconv.FormatInt(id, 10)+"-2000-6000.jpg")); err != nil {
		t.Errorf("thumbnail was not cached: %v", err)
	}
}

func TestThumbnailFallsBackToThePoster(t *testing.T) {
	e := newEnv(t, withThumbnails(t, thumbnail.Off, t.TempDir()))
	token := e.login("alice")
	id := e.createClip(token, 1000, 2000)

	code, header, body := e.get("/api/clips/"+strconv.FormatInt(id, 10)+"/thumbnail", token)
	if code != 200 || header.Get("X-Cliparr-Thumbnail") != "poster" || !bytes.HasPrefix(body, []byte("\x89PNG")) {
		t.Errorf("status %d, header %q, body %q", code, header.Get("X-Cliparr-Thumbnail"), body)
	}
}

func TestThumbnailFallsBackWhenTheTimeIsPastTheVideo(t *testing.T) {
	// The test video is 10 seconds long; the clip is at 1 hour.
	e := newEnvWithFake(t, fakejellyfin.New(fakejellyfin.WithVideo(testutil.MakeVideo(t))), withThumbnails(t, "", t.TempDir()))
	token := e.login("alice")
	id := e.createClip(token, 3_600_000, 3_660_000)

	code, header, _ := e.get("/api/clips/"+strconv.FormatInt(id, 10)+"/thumbnail", token)
	if code != 200 || header.Get("X-Cliparr-Thumbnail") != "poster" {
		t.Errorf("status %d, header %q; want the poster fallback", code, header.Get("X-Cliparr-Thumbnail"))
	}
}

func TestThumbnailBelongsToTheClipOwner(t *testing.T) {
	e := newEnv(t, withThumbnails(t, thumbnail.Off, t.TempDir()))
	id := e.createClip(e.login("alice"), 1000, 2000)

	if code, _, _ := e.get("/api/clips/"+strconv.FormatInt(id, 10)+"/thumbnail", e.login("bob")); code != 404 {
		t.Errorf("bob got status %d for alice's thumbnail, want 404", code)
	}
	if code, _, _ := e.get("/api/clips/"+strconv.FormatInt(id, 10)+"/thumbnail", ""); code != 401 {
		t.Errorf("anonymous got status %d, want 401", code)
	}
}

func TestEditingOrDeletingAClipDropsItsThumbnail(t *testing.T) {
	dir := t.TempDir()
	e := newEnvWithFake(t, fakejellyfin.New(fakejellyfin.WithVideo(testutil.MakeVideo(t))), withThumbnails(t, "", dir))
	token := e.login("alice")
	id := e.createClip(token, 1000, 3000)
	path := "/api/clips/" + strconv.FormatInt(id, 10)
	files := func() []string { m, _ := filepath.Glob(filepath.Join(dir, "*.jpg")); return m }

	e.get(path+"/thumbnail", token)
	if len(files()) != 1 {
		t.Fatalf("expected one cached thumbnail, found %v", files())
	}
	e.call("PUT", path, token, map[string]any{"startMs": 4000, "endMs": 6000}, nil)
	if len(files()) != 0 {
		t.Errorf("editing the clip left %v behind", files())
	}

	e.get(path+"/thumbnail", token)
	e.call("DELETE", path, token, nil, nil)
	if len(files()) != 0 {
		t.Errorf("deleting the clip left %v behind", files())
	}
}
