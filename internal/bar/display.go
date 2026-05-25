//go:build darwin

package bar

import (
	"fmt"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

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

func latestRPS(window []float64) float64 {
	if len(window) == 0 {
		return 0
	}
	return window[len(window)-1]
}
