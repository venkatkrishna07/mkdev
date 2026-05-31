//go:build !windows

package bar

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func terminateProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}

// processIsMkdevBar reports whether the running process at pid looks like
// `mkdev bar`. Reads via `ps -o command=` (works on darwin + linux). False
// (deny) on lookup failure — safer than risking SIGTERM on a reused PID.
func processIsMkdevBar(pid int) bool {
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	cmdline := strings.ToLower(strings.TrimSpace(string(out)))
	if cmdline == "" {
		return false
	}
	if !strings.Contains(cmdline, "mkdev") {
		return false
	}
	return strings.Contains(cmdline, " bar") || strings.HasSuffix(cmdline, " bar") || strings.Contains(cmdline, "/bar ")
}
