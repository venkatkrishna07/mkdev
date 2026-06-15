package bar

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SpawnIfNeeded launches `mkdev bar` detached when a GUI session is present
// and no other bar instance is running. Safe to call repeatedly — the
// idempotent file lock prevents duplicate launches. Returns nil on no-op.
func SpawnIfNeeded() error {
	if !hasGUISession() {
		slog.Debug("bar: skip spawn, no GUI session")
		return nil
	}
	if barLocked() {
		slog.Debug("bar: skip spawn, already running")
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	cmd := exec.Command(exe, "bar") //nolint:gosec
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn bar: %w", err)
	}
	return cmd.Process.Release()
}

func barLocked() bool {
	home, err := homeDir()
	if err != nil {
		slog.Warn("bar: homeDir lookup failed, skipping spawn", "err", err)
		return true
	}
	data, err := os.ReadFile(filepath.Join(home, "bar.lock")) //nolint:gosec
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	return processAlive(pid)
}
