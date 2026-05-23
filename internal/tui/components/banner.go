package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// Banner renders a compact single-line app banner: accent bar + name +
// version on the left, status pill flush right. No box, no border.
func Banner(th styles.Theme, version, pill string, width int) string {
	pillW := lipgloss.Width(pill)
	if width <= pillW+2 {
		return pill
	}
	left := th.Title.Render("▎ ") + th.Title.Render("mkdev")
	tag := versionTag(version)
	if width >= lipgloss.Width(left)+len(" · "+tag)+pillW+2 {
		left += th.Dim.Render(" · " + tag)
	}
	leftW := lipgloss.Width(left)
	gap := max(width-leftW-pillW, 1)
	return left + strings.Repeat(" ", gap) + pill
}

// versionTag returns the version with exactly one leading "v".
func versionTag(version string) string {
	if version == "" {
		return "v?"
	}
	if version[0] == 'v' || version[0] == 'V' {
		return version
	}
	return "v" + version
}
