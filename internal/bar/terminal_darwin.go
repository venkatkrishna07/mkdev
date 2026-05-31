//go:build darwin

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
	cmdline := shellJoin(append([]string{exe}, args...))
	script := fmt.Sprintf(`tell application "Terminal" to do script %q`, cmdline)
	c := exec.Command("/usr/bin/osascript", "-e", script)
	if err := c.Start(); err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	return c.Process.Release()
}
