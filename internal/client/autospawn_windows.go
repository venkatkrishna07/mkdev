//go:build windows

package client

import "os/exec"

// startDetached starts cmd as a detached background process on Windows.
// Pragmatic for now: cmd.Start without HideWindow tweaks. Future work can
// add CREATE_NEW_PROCESS_GROUP via syscall.SysProcAttr.
func startDetached(cmd *exec.Cmd) error {
	return cmd.Start()
}
