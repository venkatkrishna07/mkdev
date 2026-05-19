package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// Issuer mints leaf certs signed by the CA, caching by SNI hostname.
type Issuer struct {
	ca        *CA
	knownHost func(string) bool
	mu        sync.RWMutex
	cache     map[string]*tls.Certificate
}

// NewIssuer creates an Issuer backed by ca. If knownHost is non-nil,
// GetCertificate refuses SNI values for which knownHost returns false —
// blocking probe-driven cert minting for unrouted domains.
func NewIssuer(ca *CA, knownHost func(string) bool) *Issuer {
	if ca == nil {
		panic("cert: NewIssuer called with nil CA")
	}
	return &Issuer{ca: ca, knownHost: knownHost, cache: map[string]*tls.Certificate{}}
}

// CACert returns the parsed root CA certificate. Safe to call from any goroutine.
func (i *Issuer) CACert() *x509.Certificate {
	return i.ca.Cert
}

// Issue returns (and caches) a leaf cert valid for host.
func (i *Issuer) Issue(host string) (*tls.Certificate, error) {
	i.mu.RLock()
	c, ok := i.cache[host]
	i.mu.RUnlock()
	if ok {
		return c, nil
	}

	leaf, err := i.mint(host)
	if err != nil {
		return nil, err
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	if existing, ok := i.cache[host]; ok {
		return existing, nil
	}
	i.cache[host] = leaf
	return leaf, nil
}

// Prune evicts cached leaf certs for hosts where known returns false. Call
// after a router refresh to bound cache growth across the daemon's lifetime.
func (i *Issuer) Prune(known func(string) bool) {
	if known == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for host := range i.cache {
		if !known(host) {
			delete(i.cache, host)
		}
	}
}

// GetCertificate is suitable as tls.Config.GetCertificate.
func (i *Issuer) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		return nil, fmt.Errorf("cert: no SNI")
	}
	if i.knownHost != nil && !i.knownHost(hello.ServerName) {
		return nil, fmt.Errorf("cert: unknown host %q", hello.ServerName)
	}
	return i.Issue(hello.ServerName)
}

func (i *Issuer) mint(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cert: gen leaf key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cert: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(180 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, i.ca.Cert, &key.PublicKey, i.ca.Key)
	if err != nil {
		return nil, fmt.Errorf("cert: sign leaf: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}, nil
}
