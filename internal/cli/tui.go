package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/tui"
)

// launchTUI is the shared entry used by both `mkdev` (no args) and
// `mkdev tui` / `mkdev serve`. It redirects slog to a file so log lines
// don't scroll the altscreen, builds a Runtime, and blocks on the program.
func launchTUI(cmd *cobra.Command) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(filepath.Join(home, "ca", "rootCA.pem")); os.IsNotExist(statErr) {
		fmt.Fprintln(os.Stderr, "\x1b[33mmkdev: no root CA found.\x1b[0m")
		fmt.Fprintln(os.Stderr, "Run `mkdev install` first to generate and trust the local CA.")
		return fmt.Errorf("no root CA at %s", filepath.Join(home, "ca", "rootCA.pem"))
	}

	prior := slog.Default()
	defer slog.SetDefault(prior)

	logPath := filepath.Join(home, "logs", "tui.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("tui: mkdir logs: %w", err)
	}
	// logPath is built from validated state-dir + literal.
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("tui: open log: %w", err)
	}
	defer func() { _ = f.Close() }()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Writer(f), &slog.HandlerOptions{Level: slog.LevelInfo})))

	rt, err := tui.NewRuntime(cmd.Context(), home)
	if err != nil {
		return err
	}
	return tui.Run(rt)
}

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the mkdev TUI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return launchTUI(cmd)
		},
	}
}
