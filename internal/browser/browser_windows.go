//go:build windows

package browser

import "os/exec"

func open(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() //nolint:gosec
}
