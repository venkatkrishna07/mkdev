//go:build linux

package bar

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// desktopEscapeExec quotes a path for a .desktop Exec= field per the spec:
// https://specifications.freedesktop.org/desktop-entry-spec/latest/exec-variables.html
// Wraps in double quotes and escapes \, `, $, ".
func desktopEscapeExec(s string) string {
	r := strings.NewReplacer(`\`, `\\\\`, "`", "\\`", "$", `\\$`, `"`, `\\"`)
	return `"` + r.Replace(s) + `"`
}

const desktopFileName = "mkdev-bar.desktop"

func autostartDesktopPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", desktopFileName), nil
}

func autostartEnabled() bool {
	p, err := autostartDesktopPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func installAutostart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	p, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir autostart: %w", err)
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=mkdev menu bar
Comment=Local HTTPS dev proxy status
Exec=%s bar
X-GNOME-Autostart-enabled=true
Terminal=false
`, desktopEscapeExec(exe))
	return os.WriteFile(p, []byte(entry), 0o600) //nolint:gosec
}

func uninstallAutostart() error {
	p, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove autostart entry: %w", err)
	}
	return nil
}
