package thumbnail_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bergercookie/cliparr/server/internal/testutil"
	"github.com/bergercookie/cliparr/server/internal/thumbnail"
)

var testTime = time.Unix(0, 0)

// videoServer serves the video and counts requests; it insists on one Authorization value.
func videoServer(t *testing.T, video, wantAuth string) (url string, requests *atomic.Int32) {
	t.Helper()
	data, err := os.ReadFile(video)
	if err != nil {
		t.Fatal(err)
	}
	requests = &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != wantAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.ServeContent(w, r, "video.webm", testTime, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, requests
}

func TestEnsureMakesAJPEGAndCachesIt(t *testing.T) {
	video := testutil.MakeVideo(t)
	url, requests := videoServer(t, video, "MediaBrowser Token=\"secret\"")
	maker, err := thumbnail.New("", t.TempDir())
	if err != nil || !maker.Enabled() {
		t.Fatalf("maker: %v, enabled %v", err, maker.Enabled())
	}

	path := maker.Path(7, 4000, 6000)
	src := thumbnail.Source{URL: url, AuthHeader: "MediaBrowser Token=\"secret\""}
	if err := maker.Ensure(t.Context(), path, src, 5000); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		t.Errorf("not a JPEG: starts with % x", data[:4])
	}

	first := requests.Load()
	if err := maker.Ensure(t.Context(), path, src, 5000); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != first {
		t.Error("second call fetched the video again instead of using the cache")
	}
}

func TestEnsureSendsTheAuthorizationHeader(t *testing.T) {
	video := testutil.MakeVideo(t)
	url, _ := videoServer(t, video, "MediaBrowser Token=\"right\"")
	maker, _ := thumbnail.New("", t.TempDir())

	err := maker.Ensure(t.Context(), maker.Path(1, 0, 1000), thumbnail.Source{URL: url, AuthHeader: "MediaBrowser Token=\"wrong\""}, 500)
	if err == nil {
		t.Fatal("expected an error when the server rejects the credentials")
	}
	if got := err.Error(); bytes.Contains([]byte(got), []byte("wrong")) {
		t.Errorf("error message leaks the token: %q", got)
	}
}

func TestEnsureFailsPastTheEndOfTheVideo(t *testing.T) {
	video := testutil.MakeVideo(t)
	url, _ := videoServer(t, video, "x")
	maker, _ := thumbnail.New("", t.TempDir())

	path := maker.Path(1, 0, 1000)
	if err := maker.Ensure(t.Context(), path, thumbnail.Source{URL: url, AuthHeader: "x"}, 600_000); err == nil {
		t.Error("expected an error for a time past the end")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a failed attempt left a file behind")
	}
}

func TestRemoveDeletesOnlyThatClipsPictures(t *testing.T) {
	dir := t.TempDir()
	maker, _ := thumbnail.New("", dir)
	for _, name := range []string{"5-0-1000.jpg", "5-2000-3000.jpg", "50-0-1000.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	maker.Remove(5)

	left, _ := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if len(left) != 1 || filepath.Base(left[0]) != "50-0-1000.jpg" {
		t.Errorf("unexpected files left: %v", left)
	}
}

func TestDisabledMaker(t *testing.T) {
	for _, setting := range []string{thumbnail.Off, "definitely-not-installed-ffmpeg"} {
		maker, err := thumbnail.New(setting, t.TempDir())
		if err != nil || maker.Enabled() {
			t.Errorf("%q: err %v, enabled %v; want disabled", setting, err, maker.Enabled())
		}
	}
}
