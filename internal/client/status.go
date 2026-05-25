package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

func (c *Client) Shutdown(ctx context.Context) error {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/shutdown", nil)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		return decodeError(resp)
	}
	return nil
}

func (c *Client) Status(ctx context.Context) (api.Status, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/status", nil)
	if err != nil {
		return api.Status{}, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return api.Status{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return api.Status{}, decodeError(resp)
	}
	var out api.Status
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.Status{}, fmt.Errorf("client: decode status: %w", err)
	}
	return out, nil
}
