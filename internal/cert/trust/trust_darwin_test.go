//go:build darwin

package trust_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
)

func TestInstallUninstallKeychain(t *testing.T) {
	if os.Getenv("MKDEV_TEST_KEYCHAIN") != "1" {
		t.Skip("set MKDEV_TEST_KEYCHAIN=1 to run; will prompt for sudo and mutate system keychain")
	}
	dir := t.TempDir()
	ca, err := cert.CreateCA(dir, "mkdev test CA")
	require.NoError(t, err)

	require.NoError(t, trust.Install(filepath.Join(dir, "rootCA.pem")))
	t.Cleanup(func() { _ = trust.Uninstall(filepath.Join(dir, "rootCA.pem")) })

	ok, err := trust.IsInstalled(ca.Cert)
	require.NoError(t, err)
	require.True(t, ok)
}
