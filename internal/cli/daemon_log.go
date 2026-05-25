package cli

import (
	"log/slog"
	"os"
	"strings"
)

// configureLogLevel installs a stderr slog handler honoring MKDEV_LOG_LEVEL
// (debug | info | warn | error). Defaults to info. Called once at the top of
// `mkdev daemon serve` so the daemon's structured logs land on stderr in a
// stable format regardless of any earlier configuration by cobra hooks.
func configureLogLevel() {
	lvl := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MKDEV_LOG_LEVEL"))) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	case "", "info":
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}
