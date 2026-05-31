//go:build linux

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
	for _, term := range [][]string{
		{"x-terminal-emulator", "-e"},
		{"gnome-terminal", "--"},
		{"konsole", "-e"},
		{"alacritty", "-e"},
		{"kitty"},
		{"xterm", "-e"},
	} {
		if _, err := exec.LookPath(term[0]); err != nil {
			continue
		}
		full := append(append([]string{}, term[1:]...), append([]string{exe}, args...)...)
		c := exec.Command(term[0], full...) //nolint:gosec
		if err := c.Start(); err != nil {
			return fmt.Errorf("%s: %w", term[0], err)
		}
		return c.Process.Release()
	}
	return fmt.Errorf("no terminal emulator found (x-terminal-emulator/gnome-terminal/konsole/alacritty/kitty/xterm)")
}
