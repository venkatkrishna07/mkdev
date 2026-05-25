package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

var (
	flagVerbose bool
	flagHome    string
)

// New returns the root cobra command.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:           "mkdev",
		Short:         "Local HTTPS for your dev servers",
		Long:          "mkdev maps https://<name>.<tld> to local upstreams with auto-trusted TLS.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			lvl := slog.LevelInfo
			if flagVerbose {
				lvl = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "enable debug logging")
	root.PersistentFlags().StringVar(&flagHome, "home", "", "override config dir (default ~/.mkdev)")

	tui := newTUICmd()
	root.AddCommand(
		newAddCmd(),
		newRemoveCmd(),
		newListCmd(),
		newServeCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newHostsHelperCmd(),
		tui,
		newDaemonCmd(),
		newBarCmd(),
		newVersionCmd(),
		newCompletionCmd(),
	)
	root.RunE = tui.RunE
	return root
}

// Execute runs the root command and returns its exit code.
func Execute() int {
	if err := New().Execute(); err != nil {
		if errors.Is(err, store.ErrLocked) {
			fmt.Fprintln(os.Stderr, "mkdev: another instance is already running (state.db is locked).")
			fmt.Fprintln(os.Stderr, "       quit the running instance and try again.")
			return 1
		}
		slog.Error("command failed", "err", err)
		return 1
	}
	return 0
}

// HomeDir returns the resolved config directory. Honors --home and $MKDEV_HOME.
func HomeDir() (string, error) {
	if flagHome != "" {
		return flagHome, nil
	}
	if env := os.Getenv("MKDEV_HOME"); env != "" {
		return env, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".mkdev"), nil
}
