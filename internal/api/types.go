package api

import "time"

type Share string

const (
	ShareNone Share = "none"
	ShareLAN  Share = "lan"
)

type Health string

const (
	HealthUnknown Health = "unknown"
	HealthProbing Health = "probing"
	HealthUp      Health = "up"
	HealthDown    Health = "down"
)

type Route struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	Share    Share  `json:"share"`
	Health   Health `json:"health"`
	Insecure bool   `json:"insecure"`
	Enabled  bool   `json:"enabled"`
}

type RouteStats struct {
	LastSeen time.Time `json:"last_seen"`
	Health   Health    `json:"health"`
}

type Stats struct {
	Tick   time.Time             `json:"tick"`
	Total  uint64                `json:"total"`
	RPS    []float64             `json:"rps"`
	Routes map[string]RouteStats `json:"routes"`
}

type Status struct {
	Version    string    `json:"version"`
	APIVersion string    `json:"api_version"`
	PID        int       `json:"pid"`
	Uptime     string    `json:"uptime"`
	CertReady  bool      `json:"cert_ready"`
	StartedAt  time.Time `json:"started_at"`
	TLD        string    `json:"tld"`
	ProxyPort  int       `json:"proxy_port"`
}
