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
	flag.Parse()
	log.Printf("fake jellyfin listening on %s (users: alice, bob; password: pw)", *addr)
	srv := &http.Server{Addr: *addr, Handler: fakejellyfin.New(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
