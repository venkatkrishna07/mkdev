//go:build windows

package daemon

import "syscall"

const stillActive uint32 = 259

func processAlive(pid int) bool {
	const access = syscall.PROCESS_QUERY_INFORMATION
	h, err := syscall.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
