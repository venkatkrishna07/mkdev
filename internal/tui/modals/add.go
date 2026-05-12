package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// AddPayload is delivered in the Result on submit.
type AddPayload struct {
	Domain string
	Target string
	TLD    string
}

// AddKeys are the keybindings advertised by the Add modal.
type AddKeys struct {
	Tab    key.Binding
	Enter  key.Binding
	Cancel key.Binding
}

// ShortHelp implements help.KeyMap.
func (k AddKeys) ShortHelp() []key.Binding { return []key.Binding{k.Tab, k.Enter, k.Cancel} }

// FullHelp implements help.KeyMap.
func (k AddKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Tab, k.Enter, k.Cancel}}
}

// DefaultAddKeys is the default Add-modal binding set.
var DefaultAddKeys = AddKeys{
	Tab:    key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "next field")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "submit")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

// Add is the "Add Domain" modal.
type Add struct {
	th       styles.Theme
	defaultT string
	fields   [2]textinput.Model
	focus    int
}

// NewAdd constructs an empty Add modal.
func NewAdd(th styles.Theme, defaultTLD string) Add {
	name := textinput.New()
	name.Placeholder = "myapp"
	name.Focus()
	target := textinput.New()
	target.Placeholder = "localhost:3000"
	return Add{th: th, defaultT: defaultTLD, fields: [2]textinput.Model{name, target}, focus: 0}
}

// Title implements Modal.
func (a Add) Title() string { return "Add Domain" }

// Keys returns the modal's help.KeyMap.
func (a Add) Keys() help.KeyMap { return DefaultAddKeys }

// Init implements tea.Model.
func (a Add) Init() tea.Cmd { return textinput.Blink }

// Update advances the modal in response to a tea.Msg.
func (a Add) Update(msg tea.Msg) (Add, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			return a, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		case tea.KeyEnter:
			name := strings.TrimSpace(a.fields[0].Value())
			if name == "" {
				return a, nil
			}
			domain := strings.ToLower(name)
			if !strings.Contains(domain, ".") {
				domain += a.defaultT
			}
			payload := AddPayload{Domain: domain, Target: strings.TrimSpace(a.fields[1].Value()), TLD: a.defaultT}
			return a, func() tea.Msg { return Closed{Result: Result{Payload: payload}} }
		case tea.KeyTab, tea.KeyShiftTab:
			a.fields[a.focus].Blur()
			if k.Type == tea.KeyTab {
				a.focus = (a.focus + 1) % len(a.fields)
			} else {
				a.focus = (a.focus - 1 + len(a.fields)) % len(a.fields)
			}
			a.fields[a.focus].Focus()
			return a, textinput.Blink
		}
	}
	var cmd tea.Cmd
	a.fields[a.focus], cmd = a.fields[a.focus].Update(msg)
	return a, cmd
}

// View renders the Add modal.
func (a Add) View() string {
	var b strings.Builder
	b.WriteString(a.th.ModalTitle.Render("Add Domain"))
	b.WriteString("\n\n")
	b.WriteString(a.th.Dim.Render("Name"))
	b.WriteString("\n")
	b.WriteString(a.fields[0].View())
	b.WriteString("\n\n")
	b.WriteString(a.th.Dim.Render("Target (host:port)"))
	b.WriteString("\n")
	b.WriteString(a.fields[1].View())
	b.WriteString("\n\n")
	b.WriteString(a.th.Dim.Render("tab:next  enter:submit  esc:cancel"))
	return a.th.Modal.Width(50).Render(b.String())
}
