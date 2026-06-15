package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/bar"
)

func newBarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bar",
		Short: "Launch the mkdev menu-bar app",
		Long: `Launch the mkdev system-tray / menu-bar app. The bar connects to the local
daemon over ~/.mkdev/daemon.sock and shows daemon status + the current route
list. Each route exposes a "Share on LAN" toggle and an "Open in browser"
entry.

Requires the daemon to be running (mkdev daemon serve). The bar blocks the
caller; use Quit from the tray to exit. Quitting the bar leaves the daemon
untouched.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return bar.Run()
		},
	}
	cmd.AddCommand(newBarInstallLoginCmd(), newBarUninstallLoginCmd(), newBarStatusLoginCmd())
	return cmd
}

func newBarInstallLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install-login",
		Short: "Register the menu bar to launch on user login",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := bar.InstallAutostart(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "mkdev bar: autostart enabled")
			return nil
		},
	}
}

func newBarUninstallLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-login",
		Short: "Remove the menu bar login-launch registration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := bar.UninstallAutostart(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "mkdev bar: autostart disabled")
			return nil
		},
	}
}

func newBarStatusLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status-login",
		Short: "Report whether the menu bar is registered to launch on login",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if bar.AutostartEnabled() {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "enabled")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "disabled")
			}
			return nil
		},
	}
}
