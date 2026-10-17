// Package web embeds the compiled Flutter web frontend.
package web

import (
	"embed"
	"io/fs"
)

// The dist directory is filled by `just build-web`. It only contains a
// .gitkeep in a fresh checkout, so the server still builds without Flutter.
//
//go:embed all:dist
var dist embed.FS

// FS returns the frontend files rooted at the web build output.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time, so this cannot happen
	}
	return sub
}
