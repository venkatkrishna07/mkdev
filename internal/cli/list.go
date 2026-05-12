package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List domain mappings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			s, err := store.Open(filepath.Join(home, "state.db"))
			if err != nil {
				return err
			}
			defer s.Close()
			rs, err := s.ListRoutes()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(w).Encode(rs)
			}
			if len(rs) == 0 {
				// Header still emitted so machine consumers / tests can detect empty state.
				if !styled() {
					fmt.Fprintln(w, "DOMAIN\tTARGET\tENABLED\tSOURCE")
					return nil
				}
				Dim(w, "no routes — add one with `mkdev add <name> <host:port>`")
				return nil
			}
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(infoColor)
			domainStyle := lipgloss.NewStyle().Bold(true)
			okStyle := lipgloss.NewStyle().Foreground(okColor)
			offStyle := lipgloss.NewStyle().Foreground(dimColor)

			if styled() {
				fmt.Fprintln(tw,
					headerStyle.Render("DOMAIN")+"\t"+
						headerStyle.Render("TARGET")+"\t"+
						headerStyle.Render("STATUS")+"\t"+
						headerStyle.Render("SOURCE"))
			} else {
				fmt.Fprintln(tw, "DOMAIN\tTARGET\tENABLED\tSOURCE")
			}
			for _, r := range rs {
				status := "✓ up"
				st := okStyle
				if !r.Enabled {
					status = "⊘ off"
					st = offStyle
				}
				if styled() {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						domainStyle.Render(r.Domain), r.Target, st.Render(status), r.Source)
				} else {
					fmt.Fprintf(tw, "%s\t%s\t%v\t%s\n", r.Domain, r.Target, r.Enabled, r.Source)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}
