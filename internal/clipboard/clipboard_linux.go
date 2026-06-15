//go:build linux

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

func copyText(s string) error {
	for _, args := range [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	} {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
		cmd.Stdin = strings.NewReader(s)
		return cmd.Run()
	}
	return fmt.Errorf("clipboard: no wl-copy/xclip/xsel found in PATH")
}
