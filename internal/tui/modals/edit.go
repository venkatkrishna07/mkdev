package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// EditPayload is delivered in the Result on submit.
type EditPayload struct {
	Domain string
	Target string
}

// EditKeys are the keybindings advertised by the Edit modal.
type EditKeys struct {
	Save   key.Binding
	Cancel key.Binding
}

// ShortHelp implements help.KeyMap.
func (k EditKeys) ShortHelp() []key.Binding { return []key.Binding{k.Save, k.Cancel} }

// FullHelp implements help.KeyMap.
func (k EditKeys) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Save, k.Cancel}} }

// DefaultEditKeys is the default Edit-modal binding set.
var DefaultEditKeys = EditKeys{
	Save:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "save")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

// Edit is the "Edit Domain" modal — domain is read-only, only target changes.
type Edit struct {
	th     styles.Theme
	domain string
	target textinput.Model
}

// NewEdit constructs an Edit modal seeded from r.
func NewEdit(th styles.Theme, r store.Route) Edit {
	t := textinput.New()
	t.SetValue(r.Target)
	t.Focus()
	return Edit{th: th, domain: r.Domain, target: t}
}

// Title implements Modal.
func (e Edit) Title() string { return "Edit Domain" }

// Keys returns the modal's help.KeyMap.
func (e Edit) Keys() help.KeyMap { return DefaultEditKeys }

// Init implements tea.Model.
func (e Edit) Init() tea.Cmd { return textinput.Blink }

// Update advances the modal in response to a tea.Msg.
func (e Edit) Update(msg tea.Msg) (Edit, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			return e, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		case tea.KeyEnter:
			return e, func() tea.Msg {
				return Closed{Result: Result{Payload: EditPayload{Domain: e.domain, Target: strings.TrimSpace(e.target.Value())}}}
			}
		}
	}
	var cmd tea.Cmd
	e.target, cmd = e.target.Update(msg)
	return e, cmd
}

// View renders the Edit modal.
func (e Edit) View() string {
	var b strings.Builder
	b.WriteString(e.th.ModalTitle.Render("Edit Domain"))
	b.WriteString("\n\n")
	b.WriteString(e.th.Dim.Render("Domain (read-only)"))
	b.WriteString("\n")
	b.WriteString(e.domain)
	b.WriteString("\n\n")
	b.WriteString(e.th.Dim.Render("Target (host:port)"))
	b.WriteString("\n")
	b.WriteString(e.target.View())
	b.WriteString("\n\n")
	b.WriteString(e.th.Dim.Render("enter:save  esc:cancel"))
	return e.th.Modal.Width(50).Render(b.String())
}
