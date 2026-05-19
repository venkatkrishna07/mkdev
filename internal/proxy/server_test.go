package proxy_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

func TestProxyEndToEnd(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "yes")
		fmt.Fprintln(w, "hello from", r.Host)
	}))
	defer backend.Close()

	target := strings.TrimPrefix(backend.URL, "http://")

	ca, err := cert.CreateCA(t.TempDir(), "test")
	require.NoError(t, err)
	is := cert.NewIssuer(ca, nil)

	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "foo.local", Target: target, Enabled: true}})

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{GetCertificate: is.GetCertificate, MinVersion: tls.VersionTLS13})
	require.NoError(t, err)
	srv := proxy.NewServer(r, ln, nil)
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	hostPort := ln.Addr().String()
	dial := func(network, addr string) (net.Conn, error) {
		return net.Dial(network, hostPort)
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				return tls.Dial(network, hostPort, &tls.Config{RootCAs: pool, ServerName: "foo.local"})
			},
			Dial: dial,
		},
	}

	resp, err := client.Get("https://foo.local/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "hello from")
	require.Equal(t, "yes", resp.Header.Get("X-Backend"))
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:1234":    true,
		"[::1]:443":         true,
		"192.168.1.10:8443": false,
		"10.0.0.1:443":      false,
		"":                  false,
		"garbage":           false,
	}
	for in, want := range cases {
		got := proxy.IsLoopbackAddr(in)
		require.Equalf(t, want, got, "input %q", in)
	}
}

func TestProxyUnknownHost404(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "test")
	require.NoError(t, err)
	is := cert.NewIssuer(ca, nil)
	r := proxy.NewRouter()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{GetCertificate: is.GetCertificate, MinVersion: tls.VersionTLS13})
	require.NoError(t, err)
	srv := proxy.NewServer(r, ln, nil)
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	hostPort := ln.Addr().String()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				return tls.Dial(network, hostPort, &tls.Config{RootCAs: pool, ServerName: "ghost.local"})
			},
		},
	}
	resp, err := client.Get("https://ghost.local/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
