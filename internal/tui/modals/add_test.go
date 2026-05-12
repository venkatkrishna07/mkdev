package modals_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

func TestAddRendersFields(t *testing.T) {
	m := modals.NewAdd(styles.NewTheme(), ".local")
	out := m.View()
	require.Contains(t, out, "Add Domain")
	require.Contains(t, out, "Name")
	require.Contains(t, out, "Target")
}

func TestAddSubmitProducesPayload(t *testing.T) {
	m := modals.NewAdd(styles.NewTheme(), ".local")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	m2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("localhost:3000")})
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	closed, ok := msg.(modals.Closed)
	require.True(t, ok)
	require.False(t, closed.Result.Cancelled)
	p, ok := closed.Result.Payload.(modals.AddPayload)
	require.True(t, ok)
	require.Equal(t, "foo.local", p.Domain)
	require.Equal(t, "localhost:3000", p.Target)
}
