package bar

import (
	"fmt"

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

func renderBrand(s Snapshot) string {
	if s.Version == "" {
		return "mkdev"
	}
	return "mkdev v" + s.Version
}

func renderStatus(s Snapshot) string {
	if !s.DaemonUp {
		return "Status:  ✗ daemon down"
	}
	return "Status:  ● running"
}

func renderUptime(s Snapshot) string {
	if !s.DaemonUp || s.Uptime == "" {
		return "Uptime:  —"
	}
	return "Uptime:  " + s.Uptime
}

func renderTraffic(s Snapshot) string {
	if !s.DaemonUp || s.Stats.Tick.IsZero() {
		return "Traffic: —"
	}
	return fmt.Sprintf("Traffic: %d total", s.Stats.Total)
}
