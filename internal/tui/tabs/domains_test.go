package tabs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
	"github.com/venkatkrishna07/mkdev/internal/tui/tabs"
)

func TestDomainsViewHeader(t *testing.T) {
	d := tabs.NewDomains(styles.NewTheme(), 100, 24)
	out := d.View()
	require.Contains(t, out, "DOMAIN")
	require.Contains(t, out, "TARGET")
	require.Contains(t, out, "STATUS")
	require.Contains(t, out, "SOURCE")
}

func TestDomainsRoutesRefreshedPopulatesTable(t *testing.T) {
	d := tabs.NewDomains(styles.NewTheme(), 100, 24)
	d2, _ := d.Update(tui.RoutesRefreshed{Routes: []store.Route{
		{Domain: "foo.local", Target: "localhost:3000", Enabled: true, Source: "ad-hoc", AddedAt: time.Now()},
		{Domain: "bar.local", Target: "localhost:4000", Enabled: false, Source: "ad-hoc", AddedAt: time.Now()},
	}})
	out := d2.View()
	require.Contains(t, out, "foo.local")
	require.Contains(t, out, "localhost:3000")
	require.Contains(t, out, "bar.local")
}
