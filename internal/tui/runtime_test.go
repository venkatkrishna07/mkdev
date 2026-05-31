package tui_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/routes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
	})
	ln, err := net.Listen("unix", filepath.Join(home, "daemon.sock"))
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	t.Setenv("MKDEV_HOME", home)

	rt, err := tui.NewRuntime(context.Background(), home)
	require.NoError(t, err)
	defer func() { _ = rt.Close() }()
	defer rt.Cancel()

	rs, err := rt.LoadRoutes()
	require.NoError(t, err)
	require.Empty(t, rs)
}
