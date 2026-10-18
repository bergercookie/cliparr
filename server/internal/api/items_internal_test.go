package api

import (
	"testing"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

func TestAudioPlayable(t *testing.T) {
	cases := map[string]bool{
		"":       true, // nothing known: do not transcode on a guess
		"aac":    true,
		"mp3":    true,
		"opus":   true,
		"vorbis": true,
		"flac":   true,
		"ac3":    false,
		"eac3":   false,
		"dts":    false,
		"truehd": false,
	}
	for codec, want := range cases {
		if got := audioPlayable(jellyfin.Item{AudioCodec: codec}); got != want {
			t.Errorf("audioPlayable(%q) = %v, want %v", codec, got, want)
		}
	}
}
