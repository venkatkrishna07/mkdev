package components

import (
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// PillKind selects the pill color and icon.
type PillKind int

const (
	PillUp PillKind = iota
	PillDown
	PillOff
)

// StatusPill renders a colored inline status indicator. There is no
// background fill or padding — just a glyph + label in the colour matching
// the state. The second argument is the bind address when up, or a
// human-readable context (typically an error) when down.
func StatusPill(th styles.Theme, kind PillKind, ctx string) string {
	switch kind {
	case PillUp:
		body := "● up"
		if ctx != "" {
			body += " " + ctx
		}
		return th.PillUp.Render(body)
	case PillDown:
		body := "✗ down"
		if ctx != "" {
			body += " " + ctx
		}
		return th.PillDown.Render(body)
	default:
		return th.PillOff.Render("⊘ off")
	}
}
