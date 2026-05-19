package tui_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
)

func TestSnapshotDenseLayout(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ca"), 0o700))
	_, err := cert.CreateCA(filepath.Join(home, "ca"), "test")
	require.NoError(t, err)
	cfg := config.Default()
	cfg.ProxyPort = 18443
	require.NoError(t, config.Save(filepath.Join(home, "config.toml"), cfg))

	rt, err := tui.NewRuntime(context.Background(), home)
	require.NoError(t, err)
	defer func() { _ = rt.Close() }()
	defer rt.Cancel()

	m := tui.NewRootForTest(rt)
	m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // dismiss splash
	m2, _ := m1.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m2a, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}) // → Domains
	m3, _ := m2a.Update(msg.RoutesRefreshed{Routes: []store.Route{
		{Domain: "foo.local", Target: "localhost:3000", Enabled: true, Source: "ad-hoc", AddedAt: time.Now()},
		{Domain: "bar.local", Target: "localhost:4000", Enabled: false, Source: "ad-hoc", AddedAt: time.Now()},
		{Domain: "api.checkout.local", Target: "localhost:4001", Enabled: true, Source: "ad-hoc", AddedAt: time.Now()},
	}})

	out := m3.View()
	fmt.Println("=== RENDER ===")
	fmt.Println(out)
	fmt.Println("=== END ===")

	require.Contains(t, out, "mkdev")
	require.Contains(t, out, "Domains")
	require.Contains(t, out, "foo.local")
}
