//go:build darwin

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const launchdLabel = "sh.mkdev.daemon"

// UnitPath returns the absolute path of the user-scope launchd plist.
func UnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

// InstallUnit writes the launchd plist under ~/Library/LaunchAgents. The plist
// invokes `<exePath> daemon serve` with KeepAlive and RunAtLoad. Logs land in
// ~/Library/Logs/mkdev/.
func InstallUnit(exePath string) (string, error) {
	if strings.TrimSpace(exePath) == "" {
		return "", fmt.Errorf("daemon: install: exePath required")
	}
	plistPath, err := UnitPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o750); err != nil {
		return "", fmt.Errorf("daemon: install: mkdir: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(home, "Library", "Logs", "mkdev")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return "", fmt.Errorf("daemon: install: mkdir logs: %w", err)
	}
	body := renderPlist(exePath, filepath.Join(logDir, "daemon.out.log"), filepath.Join(logDir, "daemon.err.log"))
	if err := os.WriteFile(plistPath, []byte(body), 0o644); err != nil { //nolint:gosec // user plist needs 0644 for launchd
		return "", fmt.Errorf("daemon: install: write plist: %w", err)
	}
	return plistPath, nil
}

// UninstallUnit removes the plist (idempotent — missing file is OK).
func UninstallUnit() error {
	plistPath, err := UnitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("daemon: uninstall: remove plist: %w", err)
	}
	return nil
}

// EnableUnit bootstraps the agent into the current GUI session and starts it.
func EnableUnit() error {
	plistPath, err := UnitPath()
	if err != nil {
		return err
	}
	if !fileExists(plistPath) {
		return fmt.Errorf("daemon: enable: plist missing — run `mkdev daemon install` first")
	}
	uid := os.Getuid()
	// Best-effort bootout first to clear any half-loaded state; ignore error.
	_ = run("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, launchdLabel))
	if err := run("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath); err != nil {
		return fmt.Errorf("daemon: enable: launchctl bootstrap: %w", err)
	}
	if err := run("launchctl", "enable", fmt.Sprintf("gui/%d/%s", uid, launchdLabel)); err != nil {
		return fmt.Errorf("daemon: enable: launchctl enable: %w", err)
	}
	if err := run("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", uid, launchdLabel)); err != nil {
		return fmt.Errorf("daemon: enable: launchctl kickstart: %w", err)
	}
	return nil
}

// DisableUnit stops the agent and bootouts it from the session. Idempotent.
func DisableUnit() error {
	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d/%s", uid, launchdLabel)
	// Best-effort; if it isn't loaded, bootout returns error — swallow.
	_ = run("launchctl", "bootout", target)
	return nil
}

// QueryUnit returns whether the plist is installed and currently loaded.
func QueryUnit() (UnitState, error) {
	st := UnitState{}
	plistPath, err := UnitPath()
	if err != nil {
		return st, err
	}
	st.Path = plistPath
	st.Installed = fileExists(plistPath)
	uid := os.Getuid()
	if err := run("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, launchdLabel)); err == nil {
		st.Loaded = true
	}
	return st, nil
}

func renderPlist(exePath, outLog, errLog string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>` + launchdLabel + `</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + exePath + `</string>
        <string>daemon</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>5</integer>
    <key>StandardOutPath</key>
    <string>` + outLog + `</string>
    <key>StandardErrorPath</key>
    <string>` + errLog + `</string>
</dict>
</plist>
`
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // operator-supplied launchctl invocations
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
