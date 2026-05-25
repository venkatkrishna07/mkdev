//go:build darwin

package bar

import (
	"fmt"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

// healthDot returns a single-glyph status indicator for a route health.
func healthDot(h api.Health) string {
	switch h {
	case api.HealthUp:
		return "●"
	case api.HealthDown:
		return "✗"
	case api.HealthProbing:
		return "◌"
	default:
		return "○"
	}
}

// renderHeader formats the disabled top menu item that shows daemon liveness,
// route count, the current rolling RPS, and (when known) version/uptime.
func renderHeader(s Snapshot) string {
	if !s.DaemonUp {
		return "daemon: down"
	}
	parts := []string{fmt.Sprintf("daemon: up · %d routes", len(s.Routes))}
	if rps := latestRPS(s.Stats.RPS); rps > 0 {
		parts = append(parts, fmt.Sprintf("%.1f req/s", rps))
	}
	if s.Version != "" {
		parts = append(parts, "v"+s.Version)
	}
	if s.Uptime != "" {
		parts = append(parts, "up "+s.Uptime)
	}
	return strings.Join(parts, " · ")
}

// latestRPS returns the most recent per-second value from a rolling RPS slice.
func latestRPS(window []float64) float64 {
	if len(window) == 0 {
		return 0
	}
	return window[len(window)-1]
}
