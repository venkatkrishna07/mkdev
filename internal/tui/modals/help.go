package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// HelpKeys are the keybindings advertised by the Help modal.
type HelpKeys struct {
	Close key.Binding
}

// ShortHelp implements help.KeyMap.
func (k HelpKeys) ShortHelp() []key.Binding { return []key.Binding{k.Close} }

// FullHelp implements help.KeyMap.
func (k HelpKeys) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Close}} }

// DefaultHelpKeys is the default Help-modal binding set.
var DefaultHelpKeys = HelpKeys{
	Close: key.NewBinding(key.WithKeys("esc", "enter", "?"), key.WithHelp("esc/↵/?", "close")),
}

// Help is a read-only modal that lists key bindings.
type Help struct {
	th       styles.Theme
	bindings []key.Binding
}

// NewHelp constructs a Help modal listing bindings.
func NewHelp(th styles.Theme, bindings []key.Binding) Help {
	return Help{th: th, bindings: bindings}
}

// Title implements Modal.
func (h Help) Title() string { return "Help" }

// Keys returns the modal's help.KeyMap.
func (h Help) Keys() help.KeyMap { return DefaultHelpKeys }

// Init implements tea.Model.
func (h Help) Init() tea.Cmd { return nil }

// Update advances the modal in response to a tea.Msg.
func (h Help) Update(msg tea.Msg) (Help, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc, tea.KeyEnter:
			return h, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		}
		if k.String() == "?" {
			return h, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		}
	}
	return h, nil
}

// View renders the Help modal.
func (h Help) View() string {
	keyCol := 0
	for _, b := range h.bindings {
		if w := len(b.Help().Key); w > keyCol {
			keyCol = w
		}
	}
	if keyCol < 6 {
		keyCol = 6
	}

	var b strings.Builder
	b.WriteString(h.th.ModalTitle.Render("Key bindings"))
	b.WriteString("\n\n")
	for _, kb := range h.bindings {
		hk := kb.Help()
		if hk.Key == "" && hk.Desc == "" {
			continue
		}
		pad := max(keyCol-len(hk.Key), 0)
		b.WriteString(h.th.FooterKey.Render(hk.Key))
		b.WriteString(strings.Repeat(" ", pad+3))
		b.WriteString(h.th.Footer.Render(hk.Desc))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(h.th.Dim.Render("esc/↵/?  close"))
	return h.th.Modal.Width(50).Render(b.String())
}
