package modals

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

// Result is returned to the root model when a modal closes.
type Result struct {
	Cancelled bool
	Payload   any
}

// Modal is the contract for every modal screen. Each modal owns a help.KeyMap
// so the root footer can render the modal's bindings while it is on top.
type Modal interface {
	tea.Model
	Title() string
	Keys() help.KeyMap
}

// Closed is sent by a modal to its parent to indicate it should be popped.
type Closed struct{ Result Result }
