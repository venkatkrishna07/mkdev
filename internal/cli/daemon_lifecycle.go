package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
)

func newDaemonInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the OS user-service for the daemon (launchd / systemd --user)",
		Long: `Install the OS user-service unit so the daemon starts at login and is
respawned on crash. Writes a launchd plist on macOS or a systemd --user
unit on Linux. Use 'mkdev daemon enable' to load and start it.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("daemon install: resolve own exe: %w", err)
			}
			path, err := daemon.InstallUnit(exe)
			if err != nil {
				return err
			}
			Success(cmd.OutOrStdout(), "installed: "+path)
			Info(cmd.OutOrStdout(), "next: `mkdev daemon enable` to load + start")
			return nil
		},
	}
}

func newDaemonUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the OS user-service unit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := daemon.DisableUnit(); err != nil && !errors.Is(err, daemon.ErrUnitUnsupported) {
				Warn(cmd.OutOrStdout(), "disable failed (continuing): "+err.Error())
			}
			if err := daemon.UninstallUnit(); err != nil {
				return err
			}
			Success(cmd.OutOrStdout(), "uninstalled")
			return nil
		},
	}
}

func newDaemonEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Load + start the daemon via the OS supervisor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := daemon.EnableUnit(); err != nil {
				return err
			}
			Success(cmd.OutOrStdout(), "enabled — daemon should be running")
			return nil
		},
	}
}

func newDaemonDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Stop + unload the daemon from the OS supervisor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := daemon.DisableUnit(); err != nil {
				return err
			}
			Success(cmd.OutOrStdout(), "disabled")
			return nil
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Ask a running daemon to shut down (POST /v1/shutdown)",
		Long: `Send a graceful-shutdown request to a running daemon. If the OS
supervisor (launchd / systemd) is configured, it will restart the daemon
shortly after; use 'mkdev daemon disable' to stop persistently.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client.New(client.Options{})
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Second)
			defer cancel()
			if err := c.Shutdown(ctx); err != nil {
				if errors.Is(err, client.ErrDaemonDown) {
					Warn(cmd.OutOrStdout(), "daemon was not running")
					return nil
				}
				return err
			}
			Success(cmd.OutOrStdout(), "shutdown requested")
			return nil
		},
	}
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report daemon liveness, version, and supervisor state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			c, err := client.New(client.Options{})
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
			defer cancel()
			st, statusErr := c.Status(ctx)
			if statusErr != nil {
				if errors.Is(statusErr, client.ErrDaemonDown) {
					Warn(w, "daemon: not running")
				} else {
					Warn(w, "daemon status error: "+statusErr.Error())
				}
			} else {
				Success(w, fmt.Sprintf(
					"daemon: up · v%s · pid %d · uptime %s · tld %s",
					st.Version, st.PID, st.Uptime, st.TLD,
				))
				if st.CertReady {
					Info(w, "cert: ready")
				} else {
					Dim(w, "cert: not loaded (proxy may be in API-only mode; run `mkdev install` if needed)")
				}
			}
			unit, unitErr := daemon.QueryUnit()
			if unitErr != nil {
				if errors.Is(unitErr, daemon.ErrUnitUnsupported) {
					Dim(w, "unit: unsupported on this OS")
				} else {
					Dim(w, "unit query failed: "+unitErr.Error())
				}
				return nil
			}
			state := "absent"
			if unit.Installed {
				state = "installed"
				if unit.Loaded {
					state = "installed + loaded"
				}
			}
			Info(w, fmt.Sprintf("unit: %s at %s", state, unit.Path))
			if unit.Note != "" {
				Dim(w, "note: "+unit.Note)
			}
			return nil
		},
	}
}
