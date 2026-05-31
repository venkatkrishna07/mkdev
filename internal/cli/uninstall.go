package cli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/bar"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
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

			Step(w, "disabling daemon user-service…")
			if err := daemon.DisableUnit(); err != nil && !errors.Is(err, daemon.ErrUnitUnsupported) {
				Warn(w, "disable failed (continuing): "+err.Error())
			}
			if err := daemon.UninstallUnit(); err != nil && !errors.Is(err, daemon.ErrUnitUnsupported) {
				Warn(w, "daemon uninstall failed (continuing): "+err.Error())
			} else {
				Success(w, "daemon service removed")
			}

			Step(w, "stopping menu bar (if running)…")
			stopped, err := bar.Stop()
			switch {
			case err != nil:
				Warn(w, "bar stop failed (continuing): "+err.Error())
			case stopped:
				Success(w, "menu bar stopped")
			default:
				Step(w, "menu bar was not running")
			}

			Step(w, "removing menu bar autostart…")
			if err := bar.UninstallAutostart(); err != nil {
				Warn(w, "bar autostart removal failed (continuing): "+err.Error())
			} else {
				Success(w, "menu bar autostart removed")
			}

			caDir := filepath.Join(home, "ca")
			rootCA := filepath.Join(caDir, "rootCA.pem")
			uninstallTrustedCA(w, caDir, rootCA)

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
				_ = s.Close()
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
			_, _ = fmt.Fprintln(w)
			Success(w, "uninstalled")
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete config, state, certs")
	return cmd
}

func uninstallTrustedCA(w io.Writer, caDir, rootCA string) {
	if _, err := os.Stat(rootCA); err != nil {
		Step(w, "no CA file to untrust at "+rootCA)
		return
	}
	ca, err := cert.LoadCA(caDir)
	if err != nil {
		Warn(w, "load CA failed (skipping trust uninstall): "+err.Error())
		return
	}
	trusted, err := trust.IsTrusted(ca.Cert)
	if err != nil {
		Warn(w, "trust check failed (skipping uninstall): "+err.Error())
		return
	}
	if !trusted {
		Step(w, "CA already absent from trust store")
		return
	}
	Step(w, "removing CA from system trust store…")
	if err := trust.Uninstall(rootCA); err != nil {
		Warn(w, "trust uninstall failed (continuing): "+err.Error())
		return
	}
	Success(w, "CA removed from trust store")
}
