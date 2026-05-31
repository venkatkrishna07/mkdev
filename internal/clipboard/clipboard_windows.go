//go:build windows

package clipboard

import (
	"os/exec"
	"strings"
)

func copyText(s string) error {
	cmd := exec.Command("clip.exe")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
