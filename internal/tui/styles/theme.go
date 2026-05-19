package styles

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name string

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

	Primary lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Accent  lipgloss.AdaptiveColor
	Surface lipgloss.AdaptiveColor
	OnPill  lipgloss.AdaptiveColor
	OK      lipgloss.AdaptiveColor
	Bad     lipgloss.AdaptiveColor
	Warn    lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor
}

type palette struct {
	primary, accent, muted, surface, onPill, ok, bad, warn, info lipgloss.AdaptiveColor
}

var palettes = map[string]palette{
	"auto": {
		primary: ac("#3B82F6", "#60A5FA"),
		accent:  ac("#8B5CF6", "#A78BFA"),
		muted:   ac("#6B7280", "#9CA3AF"),
		surface: ac("#EEF2FF", "#1E293B"),
		onPill:  ac("#FFFFFF", "#FFFFFF"),
		ok:      ac("#10B981", "#34D399"),
		bad:     ac("#DC2626", "#F87171"),
		warn:    ac("#D97706", "#FBBF24"),
		info:    ac("#2563EB", "#60A5FA"),
	},
	"catppuccin-macchiato": {
		primary: solid("#8AADF4"),
		accent:  solid("#C6A0F6"),
		muted:   solid("#8087A2"),
		surface: solid("#363A4F"),
		onPill:  solid("#24273A"),
		ok:      solid("#A6DA95"),
		bad:     solid("#ED8796"),
		warn:    solid("#EED49F"),
		info:    solid("#91D7E3"),
	},
	"catppuccin-mocha": {
		primary: solid("#89B4FA"),
		accent:  solid("#CBA6F7"),
		muted:   solid("#7F849C"),
		surface: solid("#313244"),
		onPill:  solid("#1E1E2E"),
		ok:      solid("#A6E3A1"),
		bad:     solid("#F38BA8"),
		warn:    solid("#F9E2AF"),
		info:    solid("#94E2D5"),
	},
	"dracula": {
		primary: solid("#BD93F9"),
		accent:  solid("#FF79C6"),
		muted:   solid("#6272A4"),
		surface: solid("#44475A"),
		onPill:  solid("#282A36"),
		ok:      solid("#50FA7B"),
		bad:     solid("#FF5555"),
		warn:    solid("#F1FA8C"),
		info:    solid("#8BE9FD"),
	},
	"nord": {
		primary: solid("#88C0D0"),
		accent:  solid("#B48EAD"),
		muted:   solid("#4C566A"),
		surface: solid("#3B4252"),
		onPill:  solid("#2E3440"),
		ok:      solid("#A3BE8C"),
		bad:     solid("#BF616A"),
		warn:    solid("#EBCB8B"),
		info:    solid("#81A1C1"),
	},
	"tokyonight": {
		primary: solid("#7AA2F7"),
		accent:  solid("#BB9AF7"),
		muted:   solid("#565F89"),
		surface: solid("#1F2335"),
		onPill:  solid("#1A1B26"),
		ok:      solid("#9ECE6A"),
		bad:     solid("#F7768E"),
		warn:    solid("#E0AF68"),
		info:    solid("#7DCFFF"),
	},
}

func ac(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

func solid(hex string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: hex, Dark: hex}
}

func ThemeNames() []string {
	return []string{"auto", "catppuccin-macchiato", "catppuccin-mocha", "dracula", "nord", "tokyonight"}
}

// NewTheme returns the named theme. Unknown name falls back to catppuccin-macchiato.
// Zero-arg variant keeps the existing call sites compiling.
func NewTheme(name ...string) Theme {
	key := "catppuccin-macchiato"
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}
	p, ok := palettes[key]
	if !ok {
		key = "catppuccin-macchiato"
		p = palettes["catppuccin-macchiato"]
	}
	return Theme{
		Name:  key,
		Base:  lipgloss.NewStyle(),
		Title: lipgloss.NewStyle().Bold(true).Foreground(p.primary),

		TabActive: lipgloss.NewStyle().Bold(true).
			Foreground(p.onPill).Background(p.primary).Padding(0, 1),
		TabInactive: lipgloss.NewStyle().Foreground(p.muted).Padding(0, 1),

		Footer:    lipgloss.NewStyle().Foreground(p.muted),
		FooterKey: lipgloss.NewStyle().Bold(true).Foreground(p.accent),

		PillUp:   lipgloss.NewStyle().Bold(true).Foreground(p.ok),
		PillDown: lipgloss.NewStyle().Bold(true).Foreground(p.bad),
		PillOff:  lipgloss.NewStyle().Foreground(p.muted),

		RowSelected: lipgloss.NewStyle().Bold(true).Foreground(p.primary),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(p.accent),

		Rule: lipgloss.NewStyle().Foreground(p.muted),

		Modal: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.primary),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(p.primary),

		Border: lipgloss.RoundedBorder(),
		Dim:    lipgloss.NewStyle().Foreground(p.muted),

		Primary: p.primary,
		Muted:   p.muted,
		Accent:  p.accent,
		Surface: p.surface,
		OnPill:  p.onPill,
		OK:      p.ok,
		Bad:     p.bad,
		Warn:    p.warn,
		Info:    p.info,
	}
}
