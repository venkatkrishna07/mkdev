//go:build windows

package bar

import "golang.org/x/sys/windows"

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h) //nolint:errcheck
	ev, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		return false
	}
	return ev == uint32(windows.WAIT_TIMEOUT)
}
