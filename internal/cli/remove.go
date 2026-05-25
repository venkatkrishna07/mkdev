package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a domain mapping",
		Args:    cobra.ExactArgs(1),
		RunE:    runRemove,
	}
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := strings.ToLower(args[0])

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
	tld := st.TLD
	if tld == "" {
		tld = ".local"
	}

	rawName := strings.TrimSuffix(name, tld)
	domain := rawName + tld
	if !hosts.ValidHostname(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}

	binPath, err := os.Executable()
	if err != nil {
		return err
	}
	editor := hosts.NewEditor(binPath)
	if err := editor.Remove(domain); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}

	if err := c.RemoveRoute(ctx, rawName); err != nil {
		if addErr := editor.Add(domain); addErr != nil {
			slog.Error("inconsistent state", "domain", domain, "primary", err, "rollback", addErr)
			return errors.Join(err, fmt.Errorf("rollback: %w", addErr))
		}
		return daemonError(err)
	}

	Success(cmd.OutOrStdout(), "removed: "+domain)
	return nil
}
