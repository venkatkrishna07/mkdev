package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Generate CA, trust it, prepare state dir",
		RunE:  runInstall,
	}
}

func runInstall(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	Banner(w, "mkdev", version.Version, "local HTTPS for dev servers")
	fmt.Fprintln(w)

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
		return Errorf(w, "check keychain: %v", err)
	}
	if !ok {
		Step(w, "installing CA in macOS Keychain (you will be prompted)…")
		if err := trust.Install(filepath.Join(caDir, "rootCA.pem")); err != nil {
			return Errorf(w, "trust install: %v", err)
		}
		Success(w, "CA trusted in system keychain")
	} else {
		Step(w, "CA already trusted in keychain")
	}

	if fps, err := trust.ListMkdevCerts(); err == nil && len(fps) > 1 {
		Warn(w, fmt.Sprintf("multiple mkdev CAs found in keychain (%d); older entries may need manual cleanup", len(fps)))
		slog.Warn("multiple CAs in keychain", "count", len(fps))
	}

	fmt.Fprintln(w)
	Box(w, "install complete", "next:  mkdev add foo localhost:3000\n       mkdev")
	return nil
}
