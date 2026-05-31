//go:build windows

package bar

import (
	"fmt"
	"os"
	"os/exec"
)

func launchInTerminal(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	full := append([]string{"/C", "start", "", exe}, args...)
	c := exec.Command("cmd.exe", full...) //nolint:gosec
	if err := c.Start(); err != nil {
		return fmt.Errorf("cmd /C start: %w", err)
	}
	return c.Process.Release()
}
