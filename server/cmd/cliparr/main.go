// Command cliparr runs the Cliparr server.
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/config"
	"github.com/bergercookie/cliparr/server/internal/jellyfin"
	"github.com/bergercookie/cliparr/server/internal/store"
	"github.com/bergercookie/cliparr/server/internal/thumbnail"
	"github.com/bergercookie/cliparr/server/internal/web"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// databaseFile is the SQLite file inside the data directory.
const databaseFile = "cliparr.db"

func main() {
	// `cliparr healthcheck` lets Docker probe the server without curl in the image.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// healthcheck calls the local /healthz endpoint and returns the process exit code.
func healthcheck() int {
	_, portText, err := net.SplitHostPort(cmp.Or(os.Getenv("CLIPARR_LISTEN"), ":8080"))
	if err != nil {
		return 1
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return 1
	}
	// Always the loopback address: only the port comes from the configuration.
	target := url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Path: "/healthz"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	// As root, as the container starts, hand the data folder to PUID:PGID and stop being root.
	if err := handOverDataDir(log, cfg.DataDir, os.Getenv); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := checkWritable(cfg.DataDir, databaseFile); err != nil {
		return err
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, databaseFile))
	if err != nil {
		return err
	}
	defer st.Close()

	deviceID, err := st.DeviceID(ctx)
	if err != nil {
		return err
	}
	jf, err := jellyfin.New(cfg.JellyfinURL, deviceID, version)
	if err != nil {
		return err
	}

	thumbs, err := thumbnail.New(cfg.FFmpeg, filepath.Join(cfg.DataDir, "thumbnails"))
	if err != nil {
		return err
	}
	if !thumbs.Enabled() {
		log.Warn("ffmpeg not found or disabled: clip previews will show movie posters instead", "setting", cfg.FFmpeg)
	}

	srv := &http.Server{
		Addr: cfg.Listen,
		Handler: api.New(api.Options{
			Store: st, Jellyfin: jf, Version: version, Log: log, Web: web.FS(), ProxyAuth: cfg.ProxyAuth,
			JellyfinPublicURL: cfg.JellyfinPublicURL, Thumbnails: thumbs, DisablePasswordLogin: !cfg.PasswordLogin,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("cliparr started", "version", version, "listen", cfg.Listen, "jellyfin", cfg.JellyfinURL,
		"proxyAuth", cfg.ProxyAuth.Enabled(), "uid", os.Getuid(), "gid", os.Getgid())
	if advice := cfg.PublicURLAdvice(); advice != "" {
		log.Warn(advice)
	}
	go watchJellyfin(ctx, log, jf, cfg.JellyfinURL, 10*time.Second, !cfg.PasswordLogin)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
