package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// ShareBadge renders a compact indicator of a route's share scope:
// "LAN" in the accent colour when shared, "local" dimmed otherwise.
func ShareBadge(th styles.Theme, shared bool) string {
	if shared {
		return lipgloss.NewStyle().Bold(true).Foreground(th.Accent).Render("LAN")
	}
	return th.Dim.Render("local")
}
