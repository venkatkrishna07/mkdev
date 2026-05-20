//go:build linux

// Adapted from mkcert (https://github.com/FiloSottile/mkcert),
// Copyright 2018 The mkcert Authors, BSD-3-Clause. See LICENSE-MKCERT.

package trust

import (
	"crypto/sha1" //nolint:gosec
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/safeexec"
)

const anchorBaseName = "mkdev-rootCA"

var (
	systemTrustDir string
	systemTrustExt string
	systemTrustCmd []string
)

// ErrNoSystemTrust is returned when no supported Linux trust store was detected.
var ErrNoSystemTrust = errors.New("trust: no supported system trust store on this Linux distro")

func init() {
	switch {
	case pathExists("/etc/pki/ca-trust/source/anchors"):
		systemTrustDir = "/etc/pki/ca-trust/source/anchors"
		systemTrustExt = ".pem"
		systemTrustCmd = []string{"update-ca-trust", "extract"}
	case pathExists("/usr/local/share/ca-certificates"):
		systemTrustDir = "/usr/local/share/ca-certificates"
		systemTrustExt = ".crt"
		systemTrustCmd = []string{"update-ca-certificates"}
	case pathExists("/etc/ca-certificates/trust-source/anchors"):
		systemTrustDir = "/etc/ca-certificates/trust-source/anchors"
		systemTrustExt = ".crt"
		systemTrustCmd = []string{"trust", "extract-compat"}
	case pathExists("/usr/share/pki/trust/anchors"):
		systemTrustDir = "/usr/share/pki/trust/anchors"
		systemTrustExt = ".pem"
		systemTrustCmd = []string{"update-ca-certificates"}
	}
}

func anchorPath() string {
	return filepath.Join(systemTrustDir, anchorBaseName+systemTrustExt)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Install copies the certificate at certPath into the detected system trust
// store directory and runs the distro-specific update command. Requires sudo.
func Install(certPath string) error {
	if systemTrustCmd == nil {
		return ErrNoSystemTrust
	}
	if err := guardSelf(); err != nil {
		return err
	}
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	certPEM, err := os.ReadFile(abs) //nolint:gosec
	if err != nil {
		return fmt.Errorf("trust: read cert: %w", err)
	}

	tee := exec.Command("sudo", "tee", anchorPath()) //nolint:gosec
	tee.Stdin = strings.NewReader(string(certPEM))
	if out, err := tee.CombinedOutput(); err != nil {
		return fmt.Errorf("trust: tee %s: %w: %s", anchorPath(), err, strings.TrimSpace(string(out)))
	}

	upd := exec.Command("sudo", systemTrustCmd...) //nolint:gosec
	if out, err := upd.CombinedOutput(); err != nil {
		return fmt.Errorf("trust: %s: %w: %s", strings.Join(systemTrustCmd, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Uninstall removes the mkdev anchor cert from the system trust store and
// re-runs the distro-specific update command. Requires sudo.
func Uninstall(_ string) error {
	if systemTrustCmd == nil {
		return ErrNoSystemTrust
	}
	if err := guardSelf(); err != nil {
		return err
	}
	if !pathExists(anchorPath()) {
		return nil
	}
	rm := exec.Command("sudo", "rm", "-f", anchorPath()) //nolint:gosec
	if out, err := rm.CombinedOutput(); err != nil {
		return fmt.Errorf("trust: rm %s: %w: %s", anchorPath(), err, strings.TrimSpace(string(out)))
	}
	upd := exec.Command("sudo", systemTrustCmd...) //nolint:gosec
	if out, err := upd.CombinedOutput(); err != nil {
		return fmt.Errorf("trust: %s: %w: %s", strings.Join(systemTrustCmd, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListMkdevCerts returns the SHA-1 fingerprint(s) of any mkdev anchor cert
// currently installed in the system trust store. Empty slice if not installed.
func ListMkdevCerts() ([]string, error) {
	if systemTrustDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(anchorPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("trust: read anchor: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("trust: anchor at %s is not a PEM certificate", anchorPath())
	}
	sum := sha1.Sum(block.Bytes) //nolint:gosec
	return []string{strings.ToUpper(hex.EncodeToString(sum[:]))}, nil
}

// IsInstalled reports whether c matches the SHA-1 fingerprint of the mkdev
// anchor currently in the system trust store.
func IsInstalled(c *x509.Certificate) (bool, error) {
	if c == nil {
		return false, errors.New("trust: nil cert")
	}
	if systemTrustDir == "" {
		return false, nil
	}
	fps, err := ListMkdevCerts()
	if err != nil {
		return false, err
	}
	if len(fps) == 0 {
		return false, nil
	}
	sum := sha1.Sum(c.Raw) //nolint:gosec
	want := strings.ToUpper(hex.EncodeToString(sum[:]))
	return slices.Contains(fps, want), nil
}

func guardSelf() error {
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("trust: resolve self: %w", err)
	}
	if err := safeexec.VerifyBinPath(bin); err != nil {
		return fmt.Errorf("trust: %w", err)
	}
	return nil
}
