//go:build windows

package bar

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValueName = "mkdev-bar"

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close() //nolint:errcheck
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

func installAutostart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer func() {
		if cerr := k.Close(); cerr != nil {
			slog.Warn("bar: close Run key", "err", cerr)
		}
	}()
	cmd := fmt.Sprintf(`"%s" bar`, strings.ReplaceAll(exe, `"`, `""`))
	return k.SetStringValue(runValueName, cmd)
}

func uninstallAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open Run key: %w", err)
	}
	defer func() {
		if cerr := k.Close(); cerr != nil {
			slog.Warn("bar: close Run key", "err", cerr)
		}
	}()
	if err := k.DeleteValue(runValueName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete Run value: %w", err)
	}
	return nil
}
