package cert_test

import (
	"crypto/x509"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
)

func TestCreateAndLoadCA(t *testing.T) {
	dir := t.TempDir()
	ca, err := cert.CreateCA(dir, "mkdev local CA")
	require.NoError(t, err)
	require.NotNil(t, ca.Cert)
	require.NotNil(t, ca.Key)
	require.True(t, ca.Cert.IsCA)
	require.WithinDuration(t, time.Now().Add(10*365*24*time.Hour), ca.Cert.NotAfter, 24*time.Hour)

	loaded, err := cert.LoadCA(dir)
	require.NoError(t, err)
	require.Equal(t, ca.Cert.SerialNumber, loaded.Cert.SerialNumber)
}

func TestLoadCAMissing(t *testing.T) {
	_, err := cert.LoadCA(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

func TestCACertChain(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mkdev local CA")
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	_, err = ca.Cert.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	require.NoError(t, err)
}
