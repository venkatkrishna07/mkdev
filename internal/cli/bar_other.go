//go:build !darwin

package cli

import (
	"github.com/spf13/cobra"
)

func newBarCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "bar",
		Short:  "Launch the mkdev menu-bar app (macOS only in this release)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Errorf(cmd.OutOrStderr(), "mkdev bar is currently macOS-only")
		},
	}
}
