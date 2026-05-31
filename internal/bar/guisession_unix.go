//go:build !windows

package bar

import (
	"os"
	"runtime"
)

// hasGUISession reports whether the current process appears to be running in
// a user-visible graphical session. Used by the daemon to decide whether to
// auto-spawn the menu bar.
func hasGUISession() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	default:
		return false
	}
}
