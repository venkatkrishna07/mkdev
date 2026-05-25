// Package api defines the wire types exchanged between the mkdev daemon and
// its clients (CLI, TUI, status-bar app). Nothing in this package may import
// any other internal/* package.
package api

import "time"

// Share indicates whether a route is reachable from the LAN.
type Share string

// Share values.
const (
	ShareNone Share = "none"
	ShareLAN  Share = "lan"
)

// Health reflects the current reachability of a route's upstream target.
type Health string

// Health values.
const (
	HealthUnknown Health = "unknown"
	HealthProbing Health = "probing"
	HealthUp      Health = "up"
	HealthDown    Health = "down"
)

// Route is a single proxy route as exposed over the daemon API.
type Route struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	Share    Share  `json:"share"`
	Health   Health `json:"health"`
	Insecure bool   `json:"insecure"`
}

// Status is the daemon's self-report returned by GET /v1/status.
type Status struct {
	Version    string    `json:"version"`
	APIVersion string    `json:"api_version"`
	PID        int       `json:"pid"`
	Uptime     string    `json:"uptime"`
	CertReady  bool      `json:"cert_ready"`
	StartedAt  time.Time `json:"started_at"`
	TLD        string    `json:"tld"`
}
