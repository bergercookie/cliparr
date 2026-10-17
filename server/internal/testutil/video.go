// Package testutil holds helpers shared by tests.
package testutil

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// MakeVideo writes a 10 second WebM test video and returns its path. It skips
// the test when ffmpeg is not installed.
func MakeVideo(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	path := filepath.Join(t.TempDir(), "video.webm")
	cmd := exec.CommandContext(t.Context(), ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=10:size=640x360:rate=10",
		"-c:v", "libvpx", "-b:v", "200k", "-y", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create a test video: %v: %s", err, out)
	}
	return path
}
