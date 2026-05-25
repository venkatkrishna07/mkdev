//go:build darwin || linux

package client

import (
	"os/exec"
	"syscall"
)

// startDetached starts cmd in a new session so the spawned daemon survives
// the parent CLI's exit. On Unix this is achieved with Setsid via SysProcAttr.
func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
