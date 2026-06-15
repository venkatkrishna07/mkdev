package bar

import (
	"fmt"
	"os"
	"os/exec"
)

// spawnDetached launches the current executable with the given args as a
// detached background process. Used by menu actions that should outlive the
// bar's lifetime (e.g. starting the daemon, opening the TUI in a new shell).
func spawnDetached(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	cmd := exec.Command(exe, args...) //nolint:gosec
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	return cmd.Process.Release()
}
