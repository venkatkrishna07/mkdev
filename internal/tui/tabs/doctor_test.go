package tabs_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
	"github.com/venkatkrishna07/mkdev/internal/tui/tabs"
)

func TestDoctorRunsChecks(t *testing.T) {
	d := tabs.NewDoctor(styles.NewTheme(), t.TempDir(), nil)
	out := d.View()
	require.Contains(t, out, "state directory")
	require.Contains(t, out, "config.toml")
}
