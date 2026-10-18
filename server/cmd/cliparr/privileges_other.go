//go:build !linux

package main

import "log/slog"

// handOverDataDir does nothing outside Linux: only the container image needs it.
func handOverDataDir(*slog.Logger, string, func(string) string) error { return nil }
