package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

func SocketPath(homeDir string) string { return filepath.Join(homeDir, "daemon.sock") }

func PidfilePath(homeDir string) string { return filepath.Join(homeDir, "daemon.pid") }

func AcquirePidfile(homeDir string) (release func(), err error) {
	if err := os.MkdirAll(homeDir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: mkdir homedir: %w", err)
	}
	path := PidfilePath(homeDir)
	if data, rerr := os.ReadFile(path); rerr == nil {
		if pid, parseErr := strconv.Atoi(string(data)); parseErr == nil && processAlive(pid) {
			return nil, fmt.Errorf("daemon: already running (pid %d)", pid)
		}
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return nil, fmt.Errorf("daemon: write pidfile: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}

func OpenSocket(homeDir string) (net.Listener, error) {
	if err := os.MkdirAll(homeDir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: mkdir homedir: %w", err)
	}
	path := SocketPath(homeDir)
	if _, err := os.Stat(path); err == nil {
		if rmErr := os.Remove(path); rmErr != nil {
			return nil, fmt.Errorf("daemon: remove stale socket: %w", rmErr)
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("daemon: listen unix %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("daemon: chmod socket: %w", err)
	}
	return ln, nil
}
