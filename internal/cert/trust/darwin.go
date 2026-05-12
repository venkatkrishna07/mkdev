//go:build darwin

package trust

import (
	"crypto/sha1" //nolint:gosec // SHA1 fingerprint for cert identification only
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemKeychain = "/Library/Keychains/System.keychain"

// Install adds the CA certificate at certPath to the macOS system keychain
// as a trusted root. Shells out to `sudo security add-trusted-cert`, which
// will prompt for a password on the controlling TTY. Callers driving a TUI
// must release the terminal (or pre-authenticate sudo) before invoking.
func Install(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	cmd := exec.Command("sudo", "security", "add-trusted-cert",
		"-d", "-r", "trustRoot",
		"-p", "ssl", "-p", "basic",
		"-k", systemKeychain,
		abs,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: add-trusted-cert: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Uninstall removes the CA certificate at certPath from the system keychain.
// Shells out to `sudo security remove-trusted-cert`; same TTY caveat as Install.
func Uninstall(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	cmd := exec.Command("sudo", "security", "remove-trusted-cert", "-d", abs)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: remove-trusted-cert: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListMkdevCerts returns SHA1 fingerprints of every cert in the system
// keychain whose CommonName matches "mkdev local CA".
func ListMkdevCerts() ([]string, error) {
	cmd := exec.Command("security", "find-certificate", "-c", "mkdev local CA", "-Z", "-a", systemKeychain)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("trust: list certs: %w", err)
	}
	var fps []string
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "SHA-1 hash:") {
			continue
		}
		fps = append(fps, strings.TrimSpace(strings.TrimPrefix(line, "SHA-1 hash:")))
	}
	return fps, nil
}

// IsInstalled returns true if a cert with the same SHA1 fingerprint as c
// is present in the macOS system keychain.
func IsInstalled(c *x509.Certificate) (bool, error) {
	if c == nil {
		return false, errors.New("trust: nil cert")
	}
	sum := sha1.Sum(c.Raw) //nolint:gosec
	fp := strings.ToUpper(hex.EncodeToString(sum[:]))
	cmd := exec.Command("security", "find-certificate", "-Z", "-a", systemKeychain)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("trust: find-certificate: %w", err)
	}
	return strings.Contains(string(out), fp), nil
}
