//go:build darwin

package hosts

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/safeexec"
)

// HostsPath is the canonical path on macOS.
const HostsPath = "/etc/hosts"

// Editor mutates the system hosts file. Writes go through `sudo mkdev hosts-helper`
// or `osascript ... with administrator privileges` when useGUI is set.
type Editor struct {
	binPath string
	useGUI  bool
}

// NewEditor creates an editor that invokes the given mkdev binary via sudo.
// Use this from CLI commands where stdin/stdout are attached to a real TTY.
func NewEditor(mkdevBin string) *Editor {
	return &Editor{binPath: mkdevBin}
}

// NewGUIEditor returns an Editor that elevates via osascript's "with
// administrator privileges" prompt instead of sudo. Suitable for TUI
// contexts where the terminal is in altscreen and a sudo prompt would
// be invisible to the user.
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
	// binPath verified by safeexec; host validated by ValidHostname.
	cmd := exec.Command("sudo", e.binPath, "hosts-helper", op, host) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGUI invokes the helper through osascript's GUI password prompt. The
// inner shell command is built with %q (AppleScript-compatible double-quoted
// string for ASCII input). Hostname is regex-validated by ValidHostname and
// binPath is owner+perm validated by verifyBinPath; we additionally reject any
// stray `"` or `\` characters to keep the AppleScript literal unambiguous.
func (e *Editor) runGUI(op, host string) error {
	if strings.ContainsAny(e.binPath, "\"\\`") || strings.ContainsAny(e.binPath, " \t\n\r") {
		return fmt.Errorf("hosts: refusing to invoke osascript with unsafe binary path")
	}
	if strings.ContainsAny(host, "\"\\` \t\n\r") {
		return fmt.Errorf("hosts: refusing to invoke osascript with unsafe host argument")
	}
	inner := fmt.Sprintf("%s hosts-helper %s %s", e.binPath, op, host)
	script := fmt.Sprintf("do shell script %q with administrator privileges", inner)
	// script built from validated binPath/host with quote-injection guard above.
	cmd := exec.Command("osascript", "-e", script) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// verifyBinPath delegates to safeexec.VerifyBinPath so the same policy is
// shared across every elevated invocation (hosts editor + cert trust).
func verifyBinPath(bin string) error {
	return safeexec.VerifyBinPath(bin)
}
