package bar

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Stop() (stopped bool, err error) {
	home, err := homeDir()
	if err != nil {
		return false, err
	}
	lockPath := filepath.Join(home, "bar.lock")
	data, err := os.ReadFile(lockPath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read bar.lock: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, nil
	}
	if !processAlive(pid) {
		_ = os.Remove(lockPath)
		return false, nil
	}
	if !processIsMkdevBar(pid) {
		_ = os.Remove(lockPath)
		return false, fmt.Errorf("pid %d in bar.lock no longer belongs to mkdev bar (PID reuse); refusing to signal", pid)
	}
	if err := terminateProcess(pid); err != nil {
		return false, fmt.Errorf("terminate pid %d: %w", pid, err)
	}
	return true, nil
}
