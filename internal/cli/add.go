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
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
)

var addInsecure bool

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name> <target>",
		Short: "Map https://<name>.<tld> to a local upstream",
		Args:  cobra.ExactArgs(2),
		RunE:  runAdd,
	}
	cmd.Flags().BoolVar(&addInsecure, "insecure", false, "skip upstream TLS verification (private CAs)")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	name, target := args[0], args[1]
	name = strings.ToLower(name)

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
	if err := editor.Add(domain); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}

	_, err = c.AddRoute(ctx, api.Route{
		Name:     rawName,
		Target:   target,
		Share:    api.ShareNone,
		Insecure: addInsecure,
	})
	if err != nil {
		if remErr := editor.Remove(domain); remErr != nil {
			slog.Error("inconsistent state", "domain", domain, "primary", err, "rollback", remErr)
			return errors.Join(err, fmt.Errorf("rollback: %w", remErr))
		}
		return daemonError(err)
	}

	Success(cmd.OutOrStdout(), fmt.Sprintf("added: https://%s → %s", domain, target))
	return nil
}

// daemonError wraps client errors with a hint when the daemon is not running.
func daemonError(err error) error {
	if errors.Is(err, client.ErrDaemonDown) {
		return fmt.Errorf("%w — start it with `mkdev daemon serve`", err)
	}
	return err
}
