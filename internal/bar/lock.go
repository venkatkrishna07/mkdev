package bar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrAlreadyRunning = errors.New("bar: another instance is already running")

func acquireLock() (release func(), err error) {
	home, err := homeDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, fmt.Errorf("bar: mkdir home: %w", err)
	}
	lockPath := filepath.Join(home, "bar.lock")

	for range 2 {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // lockPath rooted under homeDir()
		if err == nil {
			if _, werr := f.WriteString(strconv.Itoa(os.Getpid())); werr != nil {
				_ = f.Close()
				_ = os.Remove(lockPath)
				return nil, fmt.Errorf("bar: write lock: %w", werr)
			}
			if cerr := f.Close(); cerr != nil {
				_ = os.Remove(lockPath)
				return nil, fmt.Errorf("bar: close lock: %w", cerr)
			}
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("bar: open lock: %w", err)
		}
		if data, rerr := os.ReadFile(lockPath); rerr == nil { //nolint:gosec
			if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid > 0 && processAlive(pid) {
				return nil, ErrAlreadyRunning
			}
		}
		if rerr := os.Remove(lockPath); rerr != nil && !os.IsNotExist(rerr) {
			return nil, fmt.Errorf("bar: clear stale lock: %w", rerr)
		}
	}
	return nil, ErrAlreadyRunning
}
