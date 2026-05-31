package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/bar"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon + menu bar + autostart state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()

			daemonState := "stopped"
			routes := 0
			c, cerr := client.New(client.Options{})
			if cerr == nil {
				defer func() { _ = c.Close() }()
				ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
				if _, err := c.Status(ctx); err == nil {
					daemonState = "running"
					if rs, err := c.Routes(ctx); err == nil {
						routes = len(rs)
					}
				}
				cancel()
			}

			svcState := "not installed"
			if st, err := daemon.QueryUnit(); err == nil {
				switch {
				case st.Installed && st.Loaded:
					svcState = "installed + running"
				case st.Installed:
					svcState = "installed + stopped"
				}
			} else if !errors.Is(err, daemon.ErrUnitUnsupported) {
				svcState = "unknown: " + err.Error()
			}

			barAutostart := "off"
			if bar.AutostartEnabled() {
				barAutostart = "on"
			}

			_, _ = fmt.Fprintf(w, "daemon:      %s (%d routes)\n", daemonState, routes)
			_, _ = fmt.Fprintf(w, "service:     %s\n", svcState)
			_, _ = fmt.Fprintf(w, "bar login:   %s\n", barAutostart)
			return nil
		},
	}
}
