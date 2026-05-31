package cli_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/cli"
)

func stubDaemon(t *testing.T, home string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Status{Version: "test", APIVersion: api.APIVersion, TLD: ".local"})
	})
	mux.HandleFunc("GET /v1/routes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]api.Route{})
	})
	ln, err := net.Listen("unix", filepath.Join(home, "daemon.sock"))
	require.NoError(t, err)
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = ln.Close()
	})
}

func TestRootHelp(t *testing.T) {
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, buf.String(), "mkdev")
}

func TestRootListsSubcommands(t *testing.T) {
	root := cli.New()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"add", "remove", "list", "serve", "install", "uninstall", "hosts-helper", "tui"} {
		require.True(t, names[want], "missing subcommand %s", want)
	}
}

func TestRootArgErrors(t *testing.T) {
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"add", "only-one-arg"})
	err := root.Execute()
	require.Error(t, err, "add with 1 arg should error")
}

func TestRemoveRequiresOneArg(t *testing.T) {
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"remove"})
	err := root.Execute()
	require.Error(t, err, "remove with 0 args should error")
}

func TestListWorksOnFreshHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MKDEV_HOME", home)
	stubDaemon(t, home)
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"list"})
	require.NoError(t, root.Execute())
	out := buf.String()
	require.Contains(t, out, "DOMAIN")
}

func TestListJSONWorksOnFreshHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MKDEV_HOME", home)
	stubDaemon(t, home)
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"list", "--json"})
	require.NoError(t, root.Execute())
	require.Equal(t, "[]\n", buf.String())
}
