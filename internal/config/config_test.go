package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/config"
)

func TestDefault(t *testing.T) {
	c := config.Default()
	require.Equal(t, ".local", c.TLD)
	require.Equal(t, 443, c.ProxyPort)
	require.Equal(t, "auto", c.Theme)
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	in := config.Default()
	in.TLD = ".test"
	in.ProxyPort = 8443
	require.NoError(t, config.Save(path, in))

	out, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := config.Load(filepath.Join(t.TempDir(), "missing.toml"))
	require.NoError(t, err)
	require.Equal(t, config.Default(), c)
}

func TestLoadMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	require.NoError(t, os.WriteFile(path, []byte("this is not = valid = toml"), 0o600))
	_, err := config.Load(path)
	require.Error(t, err)
}
