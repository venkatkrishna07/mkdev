package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/tui/components"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

func TestTabBarHighlightsActive(t *testing.T) {
	th := styles.NewTheme()
	out := components.TabBar(th, []string{"Domains", "Projects", "Logs"}, 1, 120)
	require.Contains(t, out, "Domains")
	require.Contains(t, out, "Projects")
	require.Contains(t, out, "Logs")
}

func TestBannerContainsNameAndPill(t *testing.T) {
	th := styles.NewTheme()
	out := components.Banner(th, "0.2.0", components.StatusPill(th, components.PillUp, "127.0.0.1:8443"), 120)
	require.Contains(t, out, "mkdev")
	require.Contains(t, out, "0.2.0")
	require.Contains(t, out, "127.0.0.1:8443")
}

func TestStatusPillUpDownOff(t *testing.T) {
	th := styles.NewTheme()
	require.Contains(t, components.StatusPill(th, components.PillUp, "127.0.0.1:8443"), "127.0.0.1:8443")
	down := components.StatusPill(th, components.PillDown, "")
	require.True(t, strings.Contains(strings.ToLower(down), "down") || strings.Contains(down, "✗"))
	require.NotEmpty(t, components.StatusPill(th, components.PillOff, ""))
}
