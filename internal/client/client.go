package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Options struct {
	SocketPath string

	Timeout time.Duration
}

type Client struct {
	httpc      *http.Client
	socketPath string
	baseURL    string
}

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

func (c *Client) SocketPath() string { return c.socketPath }

func (c *Client) Close() error { return nil }

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
