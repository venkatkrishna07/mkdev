package bar

import (
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

// routeBadges returns a " · X · Y" suffix for non-default route state.
// Empty when the route is enabled and local-only.
func routeBadges(r api.Route) string {
	var b []string
	if !r.Enabled {
		b = append(b, "disabled")
	}
	if r.Share == api.ShareLAN {
		b = append(b, "LAN")
	}
	if len(b) == 0 {
		return ""
	}
	return "  ·  " + strings.Join(b, " · ")
}

func renderBrand(s Snapshot) string {
	if s.Version == "" {
		return "mkdev"
	}
	return "mkdev " + versionLabel(s.Version)
}

// versionLabel ensures exactly one leading "v".
func versionLabel(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func renderStatusLine(s Snapshot) string {
	if !s.DaemonUp {
		return "Daemon stopped"
	}
	if s.Uptime == "" {
		return "Running"
	}
	return "Running · " + s.Uptime
}
