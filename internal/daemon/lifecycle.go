package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// SocketPath returns the unix socket path inside homeDir.
func SocketPath(homeDir string) string { return filepath.Join(homeDir, "daemon.sock") }

// PidfilePath returns the pidfile path inside homeDir.
func PidfilePath(homeDir string) string { return filepath.Join(homeDir, "daemon.pid") }

// AcquirePidfile writes the current pid to <homeDir>/daemon.pid after ensuring
// no live process already holds it. Returns a release func that unlinks the
// file. Caller must invoke release on shutdown.
func AcquirePidfile(homeDir string) (release func(), err error) {
	if err := os.MkdirAll(homeDir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: mkdir homedir: %w", err)
	}
	path := PidfilePath(homeDir)
	if data, rerr := os.ReadFile(path); rerr == nil { //nolint:gosec // path is daemon-owned under homeDir
		if pid, parseErr := strconv.Atoi(string(data)); parseErr == nil && processAlive(pid) {
			return nil, fmt.Errorf("daemon: already running (pid %d)", pid)
		}
		// Stale pidfile (parse error or dead pid) — reclaim by overwriting below.
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return nil, fmt.Errorf("daemon: write pidfile: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}

// OpenSocket binds a unix listener at <homeDir>/daemon.sock with 0600 perms.
// A stale socket file at that path is removed before binding.
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

// processAlive returns true if pid corresponds to a process this user can signal.
// Signal 0 is the POSIX "is the process there" probe; it does not deliver any signal.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return !errors.Is(err, os.ErrProcessDone)
}
