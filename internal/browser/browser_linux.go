//go:build linux

package browser

import "os/exec"

func open(url string) error {
	return exec.Command("xdg-open", url).Start() //nolint:gosec
}
