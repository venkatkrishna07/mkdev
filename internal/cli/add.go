package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name> <host:port>",
		Short: "Map https://<name>.<tld> to a local upstream",
		Args:  cobra.ExactArgs(2),
		RunE:  runAdd,
	}
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	name, target := args[0], args[1]
	domain := strings.ToLower(name)
	if !strings.Contains(domain, ".") {
		domain += cfg.TLD
	}
	if !hosts.ValidHostname(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}

	s, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	if existing, err := s.GetRoute(domain); err == nil {
		return fmt.Errorf("route already exists: %s → %s", existing.Domain, existing.Target)
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	binPath, err := os.Executable()
	if err != nil {
		return err
	}
	editor := hosts.NewEditor(binPath)
	if err := editor.Add(domain); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	r := store.Route{
		Domain:  domain,
		Target:  target,
		TLD:     cfg.TLD,
		Enabled: true,
		Source:  store.SourceAdHoc,
		AddedAt: time.Now().UTC(),
	}
	if err := s.PutRoute(r); err != nil {
		_ = editor.Remove(domain)
		return err
	}
	Success(cmd.OutOrStdout(), fmt.Sprintf("added: https://%s → %s", domain, target))
	return nil
}
