package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/bergercookie/cliparr/server/internal/jellyfin"
)

// watchJellyfin tells the operator, in the log, whether Jellyfin answers, so a
// wrong address shows up when Cliparr starts and not at the first login. It
// keeps trying until Jellyfin answers: in a Compose file Jellyfin often comes
// up after Cliparr, and that is no reason to give up.
//
// With only Quick Connect to sign in by (passwordsOff), it also checks that Jellyfin has
// Quick Connect on, because otherwise nobody could log in at all.
func watchJellyfin(ctx context.Context, log *slog.Logger, jf *jellyfin.Client, address string, every time.Duration, passwordsOff bool) {
	for attempt := 1; ; attempt++ {
		info, err := askJellyfin(ctx, jf)
		if err == nil {
			log.Info("jellyfin reachable", "url", address, "version", info.Version, "name", info.Name)
			if passwordsOff {
				warnIfNoQuickConnect(ctx, log, jf)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		// Once, then now and then, so a Jellyfin that stays down does not flood the log.
		if attempt == 1 || attempt%30 == 0 {
			log.Warn("cannot reach Jellyfin yet: logins will fail until it answers",
				"url", address, "err", err,
				"hint", "inside a container localhost is the container itself; use Jellyfin's LAN address or its Compose service name in CLIPARR_JELLYFIN_URL")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func askJellyfin(ctx context.Context, jf *jellyfin.Client) (jellyfin.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return jf.Info(ctx)
}

func warnIfNoQuickConnect(ctx context.Context, log *slog.Logger, jf *jellyfin.Client) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if on, err := jf.QuickConnectEnabled(ctx); err == nil && !on {
		log.Warn("CLIPARR_PASSWORD_LOGIN is false but Quick Connect is off in Jellyfin: nobody can sign in",
			"hint", "an administrator can turn Quick Connect on in the Jellyfin dashboard")
	}
}
