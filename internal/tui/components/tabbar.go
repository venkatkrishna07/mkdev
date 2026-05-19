package components

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// Tab is a single tab spec for the rich bar.
type Tab struct {
	Label string
	Icon  string
}

// NO_NERD_FONT=1 or NO_COLOR set ⇒ plain labels (no glyph column).
func useGlyphs() bool {
	return os.Getenv("NO_NERD_FONT") == "" && os.Getenv("NO_COLOR") == ""
}

// TabBar renders a single-line tab strip. Active uses TabActive (filled
// background); inactives use TabInactive.
func TabBar(th styles.Theme, labels []string, active int) string {
	tabs := make([]Tab, len(labels))
	for i, l := range labels {
		tabs[i] = Tab{Label: l}
	}
	return TabBarRich(th, tabs, active)
}

// TabBarRich draws tabs with optional per-tab icons.
func TabBarRich(th styles.Theme, tabs []Tab, active int) string {
	parts := make([]string, len(tabs))
	glyphs := useGlyphs()
	for i, t := range tabs {
		label := t.Label
		if glyphs && t.Icon != "" {
			label = t.Icon + " " + t.Label
		}
		if i == active {
			parts[i] = th.TabActive.Render(label)
		} else {
			parts[i] = th.TabInactive.Render(label)
		}
	}
	sep := lipgloss.NewStyle().Foreground(th.Muted).Render("│")
	return strings.Join(parts, sep)
}
