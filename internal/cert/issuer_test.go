package cert_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
)

func TestIssueLeafCert(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mkdev local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca, nil)

	leaf, err := is.Issue("foo.local")
	require.NoError(t, err)
	require.NotNil(t, leaf)

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	parsed, err := x509.ParseCertificate(leaf.Certificate[0])
	require.NoError(t, err)
	_, err = parsed.Verify(x509.VerifyOptions{Roots: pool, DNSName: "foo.local"})
	require.NoError(t, err)
}

func TestIssueIsCached(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mkdev local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca, nil)
	a, err := is.Issue("foo.local")
	require.NoError(t, err)
	b, err := is.Issue("foo.local")
	require.NoError(t, err)
	require.Same(t, a, b, "issuer should return the same cached *tls.Certificate")
}

func TestGetCertificateBySNI(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mkdev local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca, nil)
	hello := &tls.ClientHelloInfo{ServerName: "bar.local"}
	leaf, err := is.GetCertificate(hello)
	require.NoError(t, err)
	require.NotNil(t, leaf)
}
