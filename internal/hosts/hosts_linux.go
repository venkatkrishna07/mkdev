//go:build linux

package hosts

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/safeexec"
)

// HostsPath is the canonical path on Linux.
const HostsPath = "/etc/hosts"

// Editor mutates the system hosts file via `sudo mkdev hosts-helper` or
// `pkexec` when useGUI is set.
type Editor struct {
	binPath string
	useGUI  bool
}

// NewEditor creates an editor that invokes the given mkdev binary via sudo.
func NewEditor(mkdevBin string) *Editor {
	return &Editor{binPath: mkdevBin}
}

// NewGUIEditor returns an Editor that elevates via pkexec (Polkit) for TUI
// contexts; falls back to sudo when pkexec is absent.
func NewGUIEditor(mkdevBin string) *Editor {
	return &Editor{binPath: mkdevBin, useGUI: true}
}

// Read returns the current contents of /etc/hosts.
func (e *Editor) Read() (string, error) {
	b, err := os.ReadFile(HostsPath)
	if err != nil {
		return "", fmt.Errorf("hosts: read: %w", err)
	}
	return string(b), nil
}

// Add maps 127.0.0.1 to host if not already present. Requires elevated privileges.
func (e *Editor) Add(host string) error {
	if !ValidHostname(host) {
		return fmt.Errorf("hosts: invalid hostname %q", host)
	}
	if err := verifyBinPath(e.binPath); err != nil {
		return err
	}
	if e.useGUI {
		return e.runGUI("add", host)
	}
	return e.runSudo("add", host)
}

// Remove deletes the mapping for host. Requires elevated privileges.
func (e *Editor) Remove(host string) error {
	if !ValidHostname(host) {
		return fmt.Errorf("hosts: invalid hostname %q", host)
	}
	if err := verifyBinPath(e.binPath); err != nil {
		return err
	}
	if e.useGUI {
		return e.runGUI("remove", host)
	}
	return e.runSudo("remove", host)
}

func (e *Editor) runSudo(op, host string) error {
	cmd := exec.Command("sudo", e.binPath, "hosts-helper", op, host) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pkexec is the Polkit GUI elevator; ships on every modern desktop Linux.
// Falls back to sudo if pkexec is absent so headless TUIs over SSH still work.
// Path is hardcoded (not PATH-resolved) to prevent a user-writable PATH entry
// shadowing the real binary.
func (e *Editor) runGUI(op, host string) error {
	const elevator = "/usr/bin/pkexec"
	if _, err := os.Stat(elevator); err != nil {
		return e.runSudo(op, host)
	}
	if err := safeexec.VerifyBinPath(elevator); err != nil {
		return e.runSudo(op, host)
	}
	cmd := exec.Command(elevator, e.binPath, "hosts-helper", op, host) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func verifyBinPath(bin string) error {
	return safeexec.VerifyBinPath(bin)
}
