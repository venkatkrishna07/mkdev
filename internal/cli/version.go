package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, build date",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	}
}
