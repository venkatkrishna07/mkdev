package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func newUninstallCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Untrust CA and optionally purge state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			home, err := HomeDir()
			if err != nil {
				return err
			}
			rootCA := filepath.Join(home, "ca", "rootCA.pem")
			if _, err := os.Stat(rootCA); err == nil {
				Step(w, "removing CA from Keychain…")
				if err := trust.Uninstall(rootCA); err != nil {
					return Errorf(w, "trust uninstall: %v", err)
				}
				Success(w, "CA removed from keychain")
			} else {
				Step(w, "no CA file to untrust at "+rootCA)
			}

			if s, err := store.Open(filepath.Join(home, "state.db")); err == nil {
				binPath, execErr := os.Executable()
				if execErr == nil {
					editor := hosts.NewEditor(binPath)
					routes, listErr := s.ListRoutes()
					if listErr == nil && len(routes) > 0 {
						Step(w, fmt.Sprintf("cleaning %d hosts entries…", len(routes)))
						for _, rt := range routes {
							if remErr := editor.Remove(rt.Domain); remErr != nil {
								slog.Warn("uninstall: hosts remove failed", "domain", rt.Domain, "err", remErr)
							}
						}
					}
				}
				s.Close()
			}

			if purge {
				Step(w, "purging "+home)
				if err := os.RemoveAll(home); err != nil {
					return Errorf(w, "purge: %v", err)
				}
				Success(w, "state directory removed")
			} else {
				Info(w, "config preserved at "+home)
				slog.Info("uninstall preserved state", "home", home)
			}
			fmt.Fprintln(w)
			Success(w, "uninstalled")
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete config, state, certs")
	return cmd
}
