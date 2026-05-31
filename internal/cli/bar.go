package cli

import (
	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/bar"
)

func newBarCmd() *cobra.Command {
	return &cobra.Command{
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
}
