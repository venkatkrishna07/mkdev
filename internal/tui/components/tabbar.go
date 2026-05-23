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

// TabBar renders a single-line tab strip. width caps the output; the bar
// degrades to icon-only labels and finally to the active tab alone with
// arrow hints when the full bar would overflow.
func TabBar(th styles.Theme, labels []string, active, width int) string {
	tabs := make([]Tab, len(labels))
	for i, l := range labels {
		tabs[i] = Tab{Label: l}
	}
	return TabBarRich(th, tabs, active, width)
}

// TabBarRich draws tabs with optional per-tab icons, falling back to a
// compact form when the rendered width exceeds the available terminal width.
func TabBarRich(th styles.Theme, tabs []Tab, active, width int) string {
	glyphs := useGlyphs()
	sep := lipgloss.NewStyle().Foreground(th.Muted).Render("│")

	build := func(labels []string) string {
		parts := make([]string, len(labels))
		for i, label := range labels {
			if i == active {
				parts[i] = th.TabActive.Render(label)
			} else {
				parts[i] = th.TabInactive.Render(label)
			}
		}
		return strings.Join(parts, sep)
	}

	full := make([]string, len(tabs))
	for i, t := range tabs {
		label := t.Label
		if glyphs && t.Icon != "" {
			label = t.Icon + " " + t.Label
		}
		full[i] = label
	}
	rendered := build(full)
	if width <= 0 || lipgloss.Width(rendered) <= width {
		return rendered
	}

	if glyphs {
		iconsOnly := make([]string, len(tabs))
		anyIcon := false
		for i, t := range tabs {
			if t.Icon != "" {
				iconsOnly[i] = t.Icon
				anyIcon = true
			} else {
				iconsOnly[i] = t.Label
			}
		}
		if anyIcon {
			rendered = build(iconsOnly)
			if lipgloss.Width(rendered) <= width {
				return rendered
			}
		}
	}

	activeLabel := full[active]
	prev := th.Dim.Render("‹")
	next := th.Dim.Render("›")
	compact := prev + " " + th.TabActive.Render(activeLabel) + " " + next
	if lipgloss.Width(compact) <= width {
		return compact
	}
	return th.TabActive.Render(activeLabel)
}
