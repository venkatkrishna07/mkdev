package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
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
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	domain := strings.ToLower(args[0])
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
	if _, err := s.GetRoute(domain); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no such route: %s", domain)
		}
		return err
	}
	binPath, err := os.Executable()
	if err != nil {
		return err
	}
	editor := hosts.NewEditor(binPath)
	if err := editor.Remove(domain); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	if err := s.DeleteRoute(domain); err != nil {
		_ = editor.Add(domain)
		return err
	}
	Success(cmd.OutOrStdout(), "removed: "+domain)
	return nil
}
