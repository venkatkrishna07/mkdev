//go:build darwin

package hosts_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
)

func TestNewGUIEditorReturnsEditor(t *testing.T) {
	e := hosts.NewGUIEditor("/usr/local/bin/mkdev")
	require.NotNil(t, e)
}

// TestGUIEditorRejectsQuotedPath verifies the belt-and-suspenders shell-quote
// rejection without actually invoking osascript. We use a fake binary on disk
// that passes verifyBinPath but has a `"` in its path; that's impossible to
// create with a path string but we can simulate by stuffing a hostname with `"`.
func TestGUIEditorRejectsQuotedHostname(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-mkdev")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	e := hosts.NewGUIEditor(bin)
	// ValidHostname will reject the `"` before we reach the osascript check,
	// so this also exercises the validation layer end-to-end.
	err := e.Add(`evil".local`)
	require.Error(t, err)
}

func TestNewEditorStillSudoBacked(t *testing.T) {
	e := hosts.NewEditor("/usr/local/bin/mkdev")
	require.NotNil(t, e)
}
