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
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/upgrade"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

var installSkipService bool

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "One-shot setup: CA + trust + daemon service + menu bar autostart",
		Long: `Run end-to-end setup so 'mkdev add' works and the menu bar appears on
login. Generates a local CA + trusts it, installs and enables the daemon
user-service, registers the menu bar to launch on login, and spawns the bar
right now if a GUI session is present.

Pass --no-service to do CA + trust setup only (legacy behavior).`,
		RunE: runInstall,
	}
	cmd.Flags().BoolVar(&installSkipService, "no-service", false, "skip daemon service + bar autostart")
	return cmd
}

func runInstall(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	Banner(w, "mkdev", version.Version, "local HTTPS for dev servers")
	_, _ = fmt.Fprintln(w)

	home, err := HomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	Step(w, "state directory ready at "+home)

	cfgPath := filepath.Join(home, "config.toml")
	if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
		if err := config.Save(cfgPath, config.Default()); err != nil {
			return err
		}
		Step(w, "wrote default config.toml")
	} else {
		Step(w, "config.toml already exists")
	}

	caDir := filepath.Join(home, "ca")
	var ca *cert.CA
	if _, statErr := os.Stat(filepath.Join(caDir, "rootCA.pem")); os.IsNotExist(statErr) {
		Step(w, "generating local CA…")
		ca, err = cert.CreateCA(caDir, "mkdev local CA")
		if err != nil {
			return Errorf(w, "create CA: %v", err)
		}
		Success(w, "CA generated at "+caDir)
	} else {
		ca, err = cert.LoadCA(caDir)
		if err != nil {
			return Errorf(w, "load CA: %v", err)
		}
		Step(w, "CA already exists")
	}

	ok, err := trust.IsInstalled(ca.Cert)
	if err != nil {
		return Errorf(w, "check trust store: %v", err)
	}
	if !ok {
		Step(w, "installing CA in system trust store (you may be prompted for credentials)…")
		if err := trust.Install(filepath.Join(caDir, "rootCA.pem")); err != nil {
			return Errorf(w, "trust install: %v", err)
		}
		Success(w, "CA trusted in system trust store")
	} else {
		Step(w, "CA already trusted in system trust store")
	}

	if fps, err := trust.ListMkdevCerts(); err == nil && len(fps) > 1 {
		Warn(w, fmt.Sprintf("multiple mkdev CAs found in trust store (%d); older entries may need manual cleanup", len(fps)))
		slog.Warn("multiple CAs in trust store", "count", len(fps))
	}

	_, _ = fmt.Fprintln(w)
	syncHostsFromStore(w, home)

	var serviceErrs int
	if !installSkipService {
		_, _ = fmt.Fprintln(w)
		serviceErrs = installServiceLayer(cmd)
	}

	if err := upgrade.WriteMarker(home, version.String()); err != nil {
		slog.Warn("install: write upgrade marker", "err", err)
	}
	_ = upgrade.ClearPending(home)

	_, _ = fmt.Fprintln(w)
	if serviceErrs > 0 {
		body := fmt.Sprintf("%d service-layer issue(s) — see log above\nnext:  mkdev add foo localhost:3000", serviceErrs)
		Box(w, "install complete (with warnings)", body)
		return fmt.Errorf("install: %d service-layer error(s)", serviceErrs)
	}
	Box(w, "install complete", "next:  mkdev add foo localhost:3000")
	return nil
}

func syncHostsFromStore(w io.Writer, home string) {
	dbPath := filepath.Join(home, "state.db")
	if _, err := os.Stat(dbPath); err != nil {
		return
	}
	s, err := store.Open(dbPath)
	if err != nil {
		Warn(w, "skip hosts sync (state.db open failed): "+err.Error())
		return
	}
	defer func() { _ = s.Close() }()
	routes, err := s.ListRoutes()
	if err != nil {
		Warn(w, "skip hosts sync (list routes failed): "+err.Error())
		return
	}
	if len(routes) == 0 {
		return
	}
	binPath, err := os.Executable()
	if err != nil {
		Warn(w, "skip hosts sync (resolve exe failed): "+err.Error())
		return
	}
	editor := hosts.NewEditor(binPath)
	added := 0
	for _, rt := range routes {
		if !rt.Enabled {
			continue
		}
		if err := editor.Add(rt.Domain); err != nil {
			slog.Warn("install: hosts re-add failed", "domain", rt.Domain, "err", err)
			continue
		}
		added++
	}
	if added > 0 {
		Step(w, fmt.Sprintf("re-asserted %d /etc/hosts entries", added))
	}
}

func installServiceLayer(cmd *cobra.Command) int {
	w := cmd.OutOrStdout()
	errs := 0

	exe, err := os.Executable()
	if err != nil {
		Warn(w, "resolve own exe: "+err.Error())
		return 1
	}

	Step(w, "installing daemon user-service…")
	if _, err := daemon.InstallUnit(exe); err != nil && !errors.Is(err, daemon.ErrUnitUnsupported) {
		Warn(w, "daemon install: "+err.Error())
		errs++
	} else if err := daemon.EnableUnit(); err != nil && !errors.Is(err, daemon.ErrUnitUnsupported) {
		Warn(w, "daemon enable: "+err.Error())
		errs++
	} else {
		Success(w, "daemon installed + running")
	}

	Step(w, "enabling menu bar autostart…")
	if err := bar.InstallAutostart(); err != nil {
		Warn(w, "bar autostart install failed: "+err.Error())
		errs++
	} else {
		Success(w, "menu bar will launch on login")
	}

	if err := bar.SpawnIfNeeded(); err != nil {
		Warn(w, "bar spawn failed: "+err.Error())
		errs++
	}
	return errs
}
