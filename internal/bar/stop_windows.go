//go:build windows

package bar

import (
	"strings"

	"golang.org/x/sys/windows"
)

func terminateProcess(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck
	return windows.TerminateProcess(h, 0)
}

func processIsMkdevBar(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h) //nolint:errcheck
	var buf [windows.MAX_PATH]uint16
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return false
	}
	name := strings.ToLower(windows.UTF16ToString(buf[:n]))
	return strings.Contains(name, "mkdev")
}
