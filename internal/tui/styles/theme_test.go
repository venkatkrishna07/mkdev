package styles_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

func TestNewTheme(t *testing.T) {
	th := styles.NewTheme()
	require.NotNil(t, th)
	require.NotEmpty(t, th.Title.Render("x"))
	require.NotEmpty(t, th.TabActive.Render("x"))
	require.NotEmpty(t, th.TabInactive.Render("x"))
	require.NotEmpty(t, th.Footer.Render("x"))
	require.NotEmpty(t, th.PillUp.Render("x"))
	require.NotEmpty(t, th.PillDown.Render("x"))
	require.NotEmpty(t, th.Rule.Render("─"))
	require.NotEmpty(t, th.Selected.Render("▸"))
	require.NotEmpty(t, th.Modal.Render("x"))

	require.True(t, th.Title.GetBold())
	require.True(t, th.TabActive.GetBold())
	require.True(t, th.FooterKey.GetBold())
	require.True(t, th.RowSelected.GetBold())
	require.True(t, th.ModalTitle.GetBold())
	require.True(t, th.Selected.GetBold())

	require.NotNil(t, th.Title.GetForeground())
	require.NotNil(t, th.PillUp.GetForeground())
	require.NotNil(t, th.PillDown.GetForeground())
	require.NotNil(t, th.PillOff.GetForeground())

	require.Equal(t, lipgloss.RoundedBorder(), th.Border)
}
