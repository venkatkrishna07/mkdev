package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Options configures a Client.
type Options struct {
	// SocketPath overrides the default unix-socket path. If empty, DefaultSocketPath() is used.
	SocketPath string
	// Timeout for individual API calls. Zero uses 10s default.
	Timeout time.Duration
}

// Client is a thin wrapper around an http.Client that talks to the
// mkdev daemon over a unix socket.
type Client struct {
	httpc      *http.Client
	socketPath string
	baseURL    string
}

// New returns a Client ready to call the daemon. It does NOT dial yet;
// the first call surfaces ErrDaemonDown if the socket is missing.
func New(opts Options) (*Client, error) {
	if opts.SocketPath == "" {
		p, err := DefaultSocketPath()
		if err != nil {
			return nil, err
		}
		opts.SocketPath = p
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	return &Client{
		httpc:      unixHTTPClient(opts.SocketPath, opts.Timeout),
		socketPath: opts.SocketPath,
		baseURL:    "http://daemon",
	}, nil
}

// SocketPath returns the resolved unix socket path the client uses.
func (c *Client) SocketPath() string { return c.socketPath }

// Close releases any client-side resources. Currently a no-op.
func (c *Client) Close() error { return nil }

// do dispatches an http.Request and returns the response. Connection-level
// failures are wrapped as ErrDaemonDown.
func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	resp, err := c.httpc.Do(req)
	if err != nil {
		if isDaemonDown(err) {
			return nil, fmt.Errorf("%w (socket %s)", ErrDaemonDown, c.socketPath)
		}
		return nil, err
	}
	return resp, nil
}
