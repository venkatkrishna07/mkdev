package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

// Routes lists all routes known to the daemon.
func (c *Client) Routes(ctx context.Context) ([]api.Route, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/routes", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	var out []api.Route
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("client: decode routes: %w", err)
	}
	return out, nil
}

// AddRoute creates a new route. The daemon validates name and target.
func (c *Client) AddRoute(ctx context.Context, r api.Route) (api.Route, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return api.Route{}, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/routes", bytes.NewReader(body))
	if err != nil {
		return api.Route{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(ctx, req)
	if err != nil {
		return api.Route{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return api.Route{}, decodeError(resp)
	}
	var out api.Route
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.Route{}, fmt.Errorf("client: decode add: %w", err)
	}
	return out, nil
}

// RemoveRoute deletes the route by name.
func (c *Client) RemoveRoute(ctx context.Context, name string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/v1/routes/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return decodeError(resp)
	}
	return nil
}

// RouteEdit is the patch payload for EditRoute. Nil fields are not modified.
type RouteEdit struct {
	Target *string    `json:"target,omitempty"`
	Share  *api.Share `json:"share,omitempty"`
}

// EditRoute applies non-nil fields to the route by name.
func (c *Client) EditRoute(ctx context.Context, name string, e RouteEdit) (api.Route, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return api.Route{}, err
	}
	req, err := http.NewRequest(http.MethodPatch, c.baseURL+"/v1/routes/"+name, bytes.NewReader(body))
	if err != nil {
		return api.Route{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(ctx, req)
	if err != nil {
		return api.Route{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return api.Route{}, decodeError(resp)
	}
	var out api.Route
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.Route{}, fmt.Errorf("client: decode edit: %w", err)
	}
	return out, nil
}

// ToggleShare flips the LAN share bit on a route.
func (c *Client) ToggleShare(ctx context.Context, name string, enabled bool) (api.Route, error) {
	body, err := json.Marshal(map[string]bool{"enabled": enabled})
	if err != nil {
		return api.Route{}, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/routes/"+name+"/share", bytes.NewReader(body))
	if err != nil {
		return api.Route{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(ctx, req)
	if err != nil {
		return api.Route{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return api.Route{}, decodeError(resp)
	}
	var out api.Route
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.Route{}, fmt.Errorf("client: decode share: %w", err)
	}
	return out, nil
}
