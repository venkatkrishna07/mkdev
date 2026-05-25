package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// EnsureRunning checks whether the daemon socket is reachable and, if not,
// forks a detached `mkdev daemon serve` process and waits up to 3s for it
// to bind. Returns nil when the socket is reachable.
//
// This is intended for one-shot CLI calls (mkdev add/list/remove) and the
// TUI/bar startup paths. It does NOT register with launchd/systemd —
// `mkdev daemon install` does that.
//
// Path resolution: uses os.Executable() so the spawn matches the caller's
// binary version exactly.
func (c *Client) EnsureRunning(ctx context.Context) error {
	if c.pingSocket(150 * time.Millisecond) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("client: resolve own exe: %w", err)
	}
	cmd := exec.Command(exe, "daemon", "serve") //nolint:gosec // exe is os.Executable()
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := startDetached(cmd); err != nil {
		return fmt.Errorf("client: spawn daemon: %w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if c.pingSocket(150 * time.Millisecond) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%w (spawned daemon did not become ready within 3s)", ErrDaemonDown)
}

// pingSocket dials the unix socket with the given timeout. True = reachable.
func (c *Client) pingSocket(timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("unix", c.socketPath)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// IsDaemonDown is a small convenience wrapper that returns true if err is
// (or wraps) ErrDaemonDown. Useful at CLI boundaries.
func IsDaemonDown(err error) bool {
	return errors.Is(err, ErrDaemonDown)
}

// Used to silence "net/http imported and not used" in this file alone when
// no symbol from net/http is referenced (kept for future autospawn variants).
var _ = http.MethodGet
