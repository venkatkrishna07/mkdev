package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// ConfirmKeys are the keybindings advertised by the Confirm modal.
type ConfirmKeys struct {
	Yes key.Binding
	No  key.Binding
}

// ShortHelp implements help.KeyMap.
func (k ConfirmKeys) ShortHelp() []key.Binding { return []key.Binding{k.Yes, k.No} }

// FullHelp implements help.KeyMap.
func (k ConfirmKeys) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Yes, k.No}} }

// DefaultConfirmKeys is the default Confirm-modal binding set.
var DefaultConfirmKeys = ConfirmKeys{
	Yes: key.NewBinding(key.WithKeys("enter", "y", "Y"), key.WithHelp("↵/y", "yes")),
	No:  key.NewBinding(key.WithKeys("esc", "n", "N"), key.WithHelp("esc/n", "no")),
}

// Confirm is a yes/no prompt. Submit (enter) returns Payload=true; cancel
// (esc/n) returns Cancelled=true.
type Confirm struct {
	th       styles.Theme
	question string
	detail   string
}

// NewConfirm constructs a confirm modal.
func NewConfirm(th styles.Theme, question, detail string) Confirm {
	return Confirm{th: th, question: question, detail: detail}
}

// Title implements Modal.
func (c Confirm) Title() string { return "Confirm" }

// Keys returns the modal's help.KeyMap.
func (c Confirm) Keys() help.KeyMap { return DefaultConfirmKeys }

// Init implements tea.Model.
func (c Confirm) Init() tea.Cmd { return nil }

// Update advances the modal in response to a tea.Msg.
func (c Confirm) Update(msg tea.Msg) (Confirm, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEnter:
			return c, func() tea.Msg { return Closed{Result: Result{Payload: true}} }
		case tea.KeyEsc:
			return c, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		}
		switch k.String() {
		case "n", "N":
			return c, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		case "y", "Y":
			return c, func() tea.Msg { return Closed{Result: Result{Payload: true}} }
		}
	}
	return c, nil
}

// View renders the Confirm modal.
func (c Confirm) View() string {
	var b strings.Builder
	b.WriteString(c.th.ModalTitle.Render("Confirm"))
	b.WriteString("\n\n")
	b.WriteString(c.question)
	b.WriteString("\n")
	if c.detail != "" {
		b.WriteString(c.th.Dim.Render(c.detail))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(c.th.Dim.Render("enter/y:yes  esc/n:no"))
	return c.th.Modal.Width(50).Render(b.String())
}
