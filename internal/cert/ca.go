package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA holds the root certificate and its private key for the mkdev local CA.
// Key holds the CA private key; callers outside this package must not persist
// or transmit it.
type CA struct {
	Cert    *x509.Certificate
	CertPEM []byte
	Key     *ecdsa.PrivateKey
	KeyPEM  []byte
}

const (
	caCertFile = "rootCA.pem"
	caKeyFile  = "rootCA-key.pem"
)

// CreateCA generates a new ECDSA P-256 CA, writes the cert (0644) and key (0400)
// to dir, and returns it. Callers must LoadCA first or ensure dir has no existing
// rootCA-key.pem — the key file is written with 0400 perms, so overwriting an
// existing key will fail with EACCES.
func CreateCA(dir, commonName string) (*CA, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cert: mkdir: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cert: gen key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cert: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"mkdev local development"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("cert: create: %w", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("cert: parse: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("cert: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(filepath.Join(dir, caCertFile), certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("cert: write cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, caKeyFile), keyPEM, 0o400); err != nil {
		return nil, fmt.Errorf("cert: write key: %w", err)
	}
	return &CA{Cert: parsed, CertPEM: certPEM, Key: key, KeyPEM: keyPEM}, nil
}

// LoadCA reads the CA cert + key from dir.
func LoadCA(dir string) (*CA, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if err != nil {
		return nil, fmt.Errorf("cert: read cert: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, caKeyFile))
	if err != nil {
		return nil, fmt.Errorf("cert: read key: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("cert: invalid cert PEM")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert: parse: %w", err)
	}
	kblock, _ := pem.Decode(keyPEM)
	if kblock == nil {
		return nil, errors.New("cert: invalid key PEM")
	}
	key, err := x509.ParseECPrivateKey(kblock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert: parse key: %w", err)
	}
	return &CA{Cert: parsed, CertPEM: certPEM, Key: key, KeyPEM: keyPEM}, nil
}
