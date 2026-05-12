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

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the mkdev TUI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}

			// Redirect slog away from stderr so log lines don't scroll the
			// altscreen TUI. Restore the prior default on exit so any further
			// CLI output (post-quit) goes back to the terminal.
			prior := slog.Default()
			defer slog.SetDefault(prior)

			logPath := filepath.Join(home, "logs", "tui.log")
			if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
				return fmt.Errorf("tui: mkdir logs: %w", err)
			}
			f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
			if err != nil {
				return fmt.Errorf("tui: open log: %w", err)
			}
			defer f.Close()
			slog.SetDefault(slog.New(slog.NewTextHandler(io.Writer(f), &slog.HandlerOptions{Level: slog.LevelInfo})))

			rt, err := tui.NewRuntime(cmd.Context(), home)
			if err != nil {
				return err
			}
			return tui.Run(rt)
		},
	}
}
