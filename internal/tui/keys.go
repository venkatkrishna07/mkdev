package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap binds the named TUI actions to their key triggers.
type KeyMap struct {
	Add     key.Binding
	Edit    key.Binding
	Delete  key.Binding
	Share   key.Binding
	Open    key.Binding
	Up      key.Binding
	Down    key.Binding
	Help    key.Binding
	Quit    key.Binding
	NextTab key.Binding
	PrevTab key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding
	Tab5    key.Binding
	Tab6    key.Binding
}

// ShortHelp returns the single-line help bindings shown in the footer.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Add, k.Edit, k.Delete, k.Share, k.Open, k.Help, k.Quit}
}

// FullHelp returns the expanded help-screen bindings grouped by row.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Open, k.Share},
		{k.Add, k.Edit, k.Delete},
		{k.NextTab, k.PrevTab},
		{k.Tab1, k.Tab2, k.Tab3, k.Tab4, k.Tab5, k.Tab6},
		{k.Help, k.Quit},
	}
}

// DefaultKeyMap is the built-in binding set used by the TUI.
var DefaultKeyMap = KeyMap{
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Share:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "share LAN")),
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "prev tab")),
	Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
	Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "domains")),
	Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "projects")),
	Tab4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "logs")),
	Tab5:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "doctor")),
	Tab6:    key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "settings")),
}
