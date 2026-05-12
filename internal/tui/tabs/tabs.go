package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Tab is the contract every TUI tab implements. Keybinds are now owned by the
// root model via tui.KeyMap / bubbles/key, so tabs only carry a title.
type Tab interface {
	tea.Model
	Title() string
}
