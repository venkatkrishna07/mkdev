package cli

import (
	"log/slog"
	"os"
	"strings"
)

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
