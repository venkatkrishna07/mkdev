package modals_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

func TestConfirmEnterReturnsTrue(t *testing.T) {
	c := modals.NewConfirm(styles.NewTheme(), "Delete foo.local?", "irreversible")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	closed := cmd().(modals.Closed)
	require.False(t, closed.Result.Cancelled)
	require.True(t, closed.Result.Payload.(bool))
}

func TestConfirmEscReturnsCancelled(t *testing.T) {
	c := modals.NewConfirm(styles.NewTheme(), "Delete?", "")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := cmd().(modals.Closed)
	require.True(t, closed.Result.Cancelled)
}
