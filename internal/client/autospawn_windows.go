//go:build windows

package client

import "os/exec"

func startDetached(cmd *exec.Cmd) error {
	return cmd.Start()
}
