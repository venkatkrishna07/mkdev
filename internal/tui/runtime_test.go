package tui_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/tui"
)

func TestNewRuntimeRefresh(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ca"), 0o700))
	_, err := cert.CreateCA(filepath.Join(home, "ca"), "test")
	require.NoError(t, err)
	cfg := config.Default()
	cfg.ProxyPort = 18443
	require.NoError(t, config.Save(filepath.Join(home, "config.toml"), cfg))

	rt, err := tui.NewRuntime(context.Background(), home)
	require.NoError(t, err)
	defer rt.Cancel()

	rs, err := rt.LoadRoutes()
	require.NoError(t, err)
	require.Empty(t, rs)
}
