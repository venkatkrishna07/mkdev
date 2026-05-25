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

func (c *Client) EnsureRunning(ctx context.Context) error {
	if c.pingSocket(150 * time.Millisecond) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("client: resolve own exe: %w", err)
	}
	cmd := exec.Command(exe, "daemon", "serve")
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

func (c *Client) pingSocket(timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("unix", c.socketPath)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func IsDaemonDown(err error) bool {
	return errors.Is(err, ErrDaemonDown)
}

var _ = http.MethodGet
