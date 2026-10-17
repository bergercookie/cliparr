// Command fakejellyfin runs the fake Jellyfin server used for development and e2e tests.
// Users: alice and bob, password "pw".
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8096", "address to listen on")
	video := flag.String("video", "", "video file (.webm or .mp4) to stream for every item instead of zero bytes")
	flag.Parse()

	var options []fakejellyfin.Option
	if *video != "" {
		options = append(options, fakejellyfin.WithVideo(*video))
	}
	log.Printf("fake jellyfin listening on %s (users: alice, bob; password: pw)", *addr)
	srv := &http.Server{Addr: *addr, Handler: fakejellyfin.New(options...), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
