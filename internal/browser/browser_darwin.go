//go:build darwin

package browser

import "os/exec"

func open(url string) error {
	return exec.Command("/usr/bin/open", url).Start() //nolint:gosec
}
