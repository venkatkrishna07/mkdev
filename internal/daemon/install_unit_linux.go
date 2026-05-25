//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdUnitName = "mkdev.service"

// UnitPath returns the absolute path of the systemd --user unit file.
func UnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

// InstallUnit writes a systemd --user unit that runs `<exePath> daemon serve`
// with restart-on-failure. Caller must enable+start via EnableUnit.
func InstallUnit(exePath string) (string, error) {
	if strings.TrimSpace(exePath) == "" {
		return "", fmt.Errorf("daemon: install: exePath required")
	}
	unitPath, err := UnitPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return "", fmt.Errorf("daemon: install: mkdir: %w", err)
	}
	body := renderUnit(exePath)
	if err := os.WriteFile(unitPath, []byte(body), 0o644); err != nil { //nolint:gosec // systemd unit needs 0644
		return "", fmt.Errorf("daemon: install: write unit: %w", err)
	}
	// Reload daemon manager so the new unit is visible. Best-effort.
	_ = run("systemctl", "--user", "daemon-reload")
	return unitPath, nil
}

// UninstallUnit removes the unit file (idempotent).
func UninstallUnit() error {
	unitPath, err := UnitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("daemon: uninstall: remove unit: %w", err)
	}
	_ = run("systemctl", "--user", "daemon-reload")
	return nil
}

// EnableUnit enables + starts the user unit.
func EnableUnit() error {
	unitPath, err := UnitPath()
	if err != nil {
		return err
	}
	if !fileExists(unitPath) {
		return fmt.Errorf("daemon: enable: unit missing — run `mkdev daemon install` first")
	}
	if err := run("systemctl", "--user", "enable", "--now", systemdUnitName); err != nil {
		return fmt.Errorf("daemon: enable: %w", err)
	}
	return nil
}

// DisableUnit stops + disables the user unit (idempotent).
func DisableUnit() error {
	// Stop first so the running daemon exits; ignore errors when not running.
	_ = run("systemctl", "--user", "disable", "--now", systemdUnitName)
	return nil
}

// QueryUnit reports whether the unit file exists and is active.
func QueryUnit() (UnitState, error) {
	st := UnitState{Note: "user services may need `loginctl enable-linger $USER` to run when not logged in"}
	unitPath, err := UnitPath()
	if err != nil {
		return st, err
	}
	st.Path = unitPath
	st.Installed = fileExists(unitPath)
	if err := run("systemctl", "--user", "is-active", "--quiet", systemdUnitName); err == nil {
		st.Loaded = true
	}
	return st, nil
}

func renderUnit(exePath string) string {
	return `[Unit]
Description=mkdev local HTTPS dev proxy daemon
After=network.target

[Service]
ExecStart=` + exePath + ` daemon serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
