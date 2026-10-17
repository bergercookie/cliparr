// Command fakeauthelia is a stand-in for Authelia's forward-auth endpoint,
// used by the end-to-end tests so they do not need a real Authelia install.
//
// It implements the same contract a reverse proxy relies on:
//   - GET /api/authz/forward-auth answers 200 plus Remote-* headers for a
//     request carrying a valid session cookie, and 401 otherwise.
//   - GET /login?user=NAME is a test helper that "logs in" by setting the cookie.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

const cookieName = "authelia_session"

func main() {
	addr := flag.String("listen", "127.0.0.1:9091", "address to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: r.URL.Query().Get("user"), Path: "/"})
	})
	mux.HandleFunc("GET /api/authz/forward-auth", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || c.Value == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Remote-User", c.Value)
		w.Header().Set("Remote-Groups", "users")
		w.Header().Set("Remote-Name", c.Value)
		w.Header().Set("Remote-Email", c.Value+"@example.com")
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("fake authelia listening on %s", *addr)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
