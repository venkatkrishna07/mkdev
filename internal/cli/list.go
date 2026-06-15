package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List domain mappings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client.New(client.Options{})
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			st, err := c.Status(ctx)
			if err != nil {
				return daemonError(err)
			}
			rs, err := c.Routes(ctx)
			if err != nil {
				return daemonError(err)
			}

			w := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(w).Encode(rs)
			}

			tld := st.TLD
			if tld == "" {
				tld = ".local"
			}

			if len(rs) == 0 {
				if !styled() {
					_, _ = fmt.Fprintln(w, "DOMAIN\tTARGET\tSHARE\tHEALTH")
					return nil
				}
				Dim(w, "no routes — add one with `mkdev add <name> <host:port>`")
				return nil
			}

			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(infoColor)
			domainStyle := lipgloss.NewStyle().Bold(true)
			lanStyle := lipgloss.NewStyle().Foreground(okColor)
			dimStyle := lipgloss.NewStyle().Foreground(dimColor)

			if styled() {
				_, _ = fmt.Fprintln(tw,
					headerStyle.Render("DOMAIN")+"\t"+
						headerStyle.Render("TARGET")+"\t"+
						headerStyle.Render("SHARE")+"\t"+
						headerStyle.Render("HEALTH"))
			} else {
				_, _ = fmt.Fprintln(tw, "DOMAIN\tTARGET\tSHARE\tHEALTH")
			}
			for _, r := range rs {
				domain := r.Name + tld
				share := string(r.Share)
				health := string(r.Health)
				if styled() {
					shareR := dimStyle.Render(share)
					if r.Share == api.ShareLAN {
						shareR = lanStyle.Render(share)
					}
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						domainStyle.Render(domain), r.Target, shareR, health)
				} else {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						domain, r.Target, share, health)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}
