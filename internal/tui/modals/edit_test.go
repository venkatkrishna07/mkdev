package modals_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

func TestEditPrePopulates(t *testing.T) {
	r := store.Route{Domain: "foo.local", Target: "localhost:3000", Enabled: true}
	m := modals.NewEdit(styles.NewTheme(), r)
	out := m.View()
	require.Contains(t, out, "foo.local")
	require.Contains(t, out, "localhost:3000")
}

func TestEditSubmitProducesPayload(t *testing.T) {
	r := store.Route{Domain: "foo.local", Target: "localhost:3000", Enabled: true}
	m := modals.NewEdit(styles.NewTheme(), r)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	closed, ok := cmd().(modals.Closed)
	require.True(t, ok)
	p, ok := closed.Result.Payload.(modals.EditPayload)
	require.True(t, ok)
	require.Equal(t, "foo.local", p.Domain)
}
