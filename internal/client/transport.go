package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultHomeDir resolves the mkdev home directory. $MKDEV_HOME overrides;
// otherwise returns $HOME/.mkdev.
func DefaultHomeDir() (string, error) {
	if v := os.Getenv("MKDEV_HOME"); v != "" {
		return v, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("client: resolve home dir: %w", err)
	}
	return filepath.Join(h, ".mkdev"), nil
}

// DefaultSocketPath returns the conventional daemon socket path.
func DefaultSocketPath() (string, error) {
	home, err := DefaultHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "daemon.sock"), nil
}

// unixHTTPClient builds an http.Client whose transport dials the given
// unix socket path for every request.
func unixHTTPClient(socketPath string, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: timeout,
	}
}
