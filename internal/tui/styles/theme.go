package styles

import "github.com/charmbracelet/lipgloss"

// Theme groups every lipgloss style the TUI renders. Styles are kept
// intentionally minimal — the dense layout draws plain text with the terminal
// itself as the outer frame.
type Theme struct {
	Base        lipgloss.Style
	Title       lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Footer      lipgloss.Style
	FooterKey   lipgloss.Style
	PillUp      lipgloss.Style
	PillDown    lipgloss.Style
	PillOff     lipgloss.Style
	RowSelected lipgloss.Style
	Selected    lipgloss.Style
	Rule        lipgloss.Style
	Modal       lipgloss.Style
	ModalTitle  lipgloss.Style
	Border      lipgloss.Border
	Dim         lipgloss.Style

	// Color tokens — re-exported so tabs/components that build their own
	// styles (e.g. bubbles/table.Styles) can stay in sync.
	Primary lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Accent  lipgloss.AdaptiveColor
	Surface lipgloss.AdaptiveColor
	OnPill  lipgloss.AdaptiveColor
}

// NewTheme returns a single theme tuned for both dark and light terminals.
// Colors are chosen for ANSI 256 compatibility.
func NewTheme() Theme {
	primary := lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"}
	accent := lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#A78BFA"}
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	surface := lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#1E293B"}
	onPill := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}
	okFG := lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"}
	badFG := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	offFG := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}

	return Theme{
		Base:  lipgloss.NewStyle(),
		Title: lipgloss.NewStyle().Bold(true).Foreground(primary),

		TabActive:   lipgloss.NewStyle().Bold(true).Foreground(primary),
		TabInactive: lipgloss.NewStyle().Foreground(muted),

		Footer:    lipgloss.NewStyle().Foreground(muted),
		FooterKey: lipgloss.NewStyle().Bold(true).Foreground(accent),

		PillUp:   lipgloss.NewStyle().Bold(true).Foreground(okFG),
		PillDown: lipgloss.NewStyle().Bold(true).Foreground(badFG),
		PillOff:  lipgloss.NewStyle().Foreground(offFG),

		RowSelected: lipgloss.NewStyle().Bold(true).Foreground(primary),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(accent),

		Rule: lipgloss.NewStyle().Foreground(muted),

		Modal: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(primary),

		Border: lipgloss.RoundedBorder(),
		Dim:    lipgloss.NewStyle().Foreground(muted),

		Primary: primary,
		Muted:   muted,
		Accent:  accent,
		Surface: surface,
		OnPill:  onPill,
	}
}
