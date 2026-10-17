// Package thumbnail makes preview pictures of clips with ffmpeg and keeps them
// on disk.
//
// ffmpeg reads the video straight from Jellyfin over HTTP and seeks with range
// requests, so only a small part of the file is transferred.
package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Width of the pictures in pixels; the height follows the aspect ratio.
	width       = 480
	makeTimeout = 30 * time.Second
	// Off turns thumbnails off when given as the ffmpeg setting.
	Off = "off"
)

// Source is where ffmpeg reads a video from.
type Source struct {
	URL        string
	AuthHeader string // value of the Authorization header
}

// Maker creates and caches thumbnails.
type Maker struct {
	ffmpeg string // path of the ffmpeg binary; empty when thumbnails are off
	dir    string

	mu    sync.Mutex
	locks map[string]*sync.Mutex // one lock per file, so a picture is made only once at a time
}

// New creates a Maker that stores pictures in dir. ffmpeg is the program name
// or path ("ffmpeg" when empty); Off disables thumbnails. If the program
// cannot be found, thumbnails are disabled too, and Enabled reports false.
func New(ffmpeg, dir string) (*Maker, error) {
	m := &Maker{dir: dir, locks: map[string]*sync.Mutex{}}
	if ffmpeg == Off {
		return m, nil
	}
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if path, err := exec.LookPath(ffmpeg); err == nil {
		m.ffmpeg = path
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create thumbnail dir: %w", err)
	}
	return m, nil
}

// Enabled reports whether ffmpeg is available.
func (m *Maker) Enabled() bool { return m.ffmpeg != "" }

// Path is where the thumbnail of a clip with the given range is stored. The
// range is part of the name, so changing a clip's times makes a fresh picture.
func (m *Maker) Path(clipID, startMs, endMs int64) string {
	return filepath.Join(m.dir, fmt.Sprintf("%d-%d-%d.jpg", clipID, startMs, endMs))
}

// Ensure returns the path of the thumbnail, creating it from src at atMs
// (milliseconds into the video) if it does not exist yet.
func (m *Maker) Ensure(ctx context.Context, path string, src Source, atMs int64) error {
	lock := m.lockFor(path)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return m.extract(ctx, path, src, atMs)
}

// Remove deletes every stored thumbnail of a clip.
func (m *Maker) Remove(clipID int64) {
	files, _ := filepath.Glob(filepath.Join(m.dir, strconv.FormatInt(clipID, 10)+"-*.jpg"))
	for _, f := range files {
		_ = os.Remove(f)
	}
}

func (m *Maker) lockFor(path string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks[path] == nil {
		m.locks[path] = &sync.Mutex{}
	}
	return m.locks[path]
}

func (m *Maker) extract(ctx context.Context, path string, src Source, atMs int64) error {
	ctx, cancel := context.WithTimeout(ctx, makeTimeout)
	defer cancel()

	// Write to a temporary name first so a half-written picture is never served.
	tmp := path + ".tmp"
	defer func() { _ = os.Remove(tmp) }()

	args := []string{
		"-nostdin", "-loglevel", "error",
		"-protocol_whitelist", "http,https,tcp,tls,crypto",
		"-headers", "Authorization: " + src.AuthHeader + "\r\n",
		"-ss", strconv.FormatFloat(float64(atMs)/1000, 'f', 3, 64), // before -i: seek with range requests
		"-i", src.URL,
		"-frames:v", "1", "-an", "-sn",
		"-vf", fmt.Sprintf("scale=%d:-2", width),
		"-q:v", "5",
		"-f", "image2", "-y", tmp,
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, m.ffmpeg, args...) //nolint:gosec // the program is configured by the admin; arguments are not shell-interpreted
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// The arguments hold the access token, so report only ffmpeg's own message.
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	info, err := os.Stat(tmp)
	if err != nil || info.Size() == 0 {
		// ffmpeg exits successfully but writes nothing when the time is past the end.
		return errors.New("ffmpeg produced no picture (is the time past the end of the video?)")
	}
	return os.Rename(tmp, path)
}
