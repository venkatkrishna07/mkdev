package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// Banner renders a compact single-line app banner: accent bar + name +
// version on the left, status pill flush right. No box, no border.
func Banner(th styles.Theme, version, pill string, width int) string {
	if width < 40 {
		width = 40
	}
	left := th.Title.Render("▎ ") + th.Title.Render("mkdev") +
		th.Dim.Render(" · v"+version)
	leftW := lipgloss.Width(left)
	pillW := lipgloss.Width(pill)
	gap := max(width-leftW-pillW, 1)
	return left + strings.Repeat(" ", gap) + pill
}
