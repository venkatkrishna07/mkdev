//go:build windows

// Adapted from mkcert (https://github.com/FiloSottile/mkcert),
// Copyright 2018 The mkcert Authors, BSD-3-Clause. See LICENSE-MKCERT.

package trust

import (
	"crypto/sha1" //nolint:gosec
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modcrypt32                           = syscall.NewLazyDLL("crypt32.dll")
	procCertAddEncodedCertificateToStore = modcrypt32.NewProc("CertAddEncodedCertificateToStore")
	procCertCloseStore                   = modcrypt32.NewProc("CertCloseStore")
	procCertDeleteCertificateFromStore   = modcrypt32.NewProc("CertDeleteCertificateFromStore")
	procCertDuplicateCertificateContext  = modcrypt32.NewProc("CertDuplicateCertificateContext")
	procCertEnumCertificatesInStore      = modcrypt32.NewProc("CertEnumCertificatesInStore")
	procCertOpenSystemStoreW             = modcrypt32.NewProc("CertOpenSystemStoreW")
)

func Install(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	der, err := readCertDER(abs)
	if err != nil {
		return err
	}
	store, err := openRootStore()
	if err != nil {
		return err
	}
	defer store.close()
	return store.addCert(der)
}

// IsTrusted reports whether c is in the Windows root store. Presence in the
// root store is the trust signal on Windows.
func IsTrusted(c *x509.Certificate) (bool, error) {
	return IsInstalled(c)
}

func Uninstall(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	der, err := readCertDER(abs)
	if err != nil {
		return err
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("trust: parse cert: %w", err)
	}
	store, err := openRootStore()
	if err != nil {
		return err
	}
	defer store.close()
	_, err = store.deleteCertsWithSerial(parsed.SerialNumber)
	return err
}

func ListMkdevCerts() ([]string, error) {
	store, err := openRootStore()
	if err != nil {
		return nil, err
	}
	defer store.close()
	return store.fingerprintsByCN("mkdev local CA")
}

func IsInstalled(c *x509.Certificate) (bool, error) {
	if c == nil {
		return false, errors.New("trust: nil cert")
	}
	sum := sha1.Sum(c.Raw) //nolint:gosec
	want := strings.ToUpper(hex.EncodeToString(sum[:]))
	fps, err := ListMkdevCerts()
	if err != nil {
		return false, err
	}
	return slices.Contains(fps, want), nil
}

func readCertDER(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("trust: read cert: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("trust: %s not a PEM certificate", path)
	}
	return block.Bytes, nil
}

type windowsRootStore uintptr

func openRootStore() (windowsRootStore, error) {
	rootStr, err := syscall.UTF16PtrFromString("ROOT")
	if err != nil {
		return 0, err
	}
	store, _, err := procCertOpenSystemStoreW.Call(0, uintptr(unsafe.Pointer(rootStr)))
	if store != 0 {
		return windowsRootStore(store), nil
	}
	return 0, fmt.Errorf("trust: open root store: %w", err)
}

func (w windowsRootStore) close() error {
	ret, _, err := procCertCloseStore.Call(uintptr(w), 0)
	if ret != 0 {
		return nil
	}
	return fmt.Errorf("trust: close root store: %w", err)
}

func (w windowsRootStore) addCert(der []byte) error {
	ret, _, err := procCertAddEncodedCertificateToStore.Call(
		uintptr(w),
		uintptr(syscall.X509_ASN_ENCODING|syscall.PKCS_7_ASN_ENCODING),
		uintptr(unsafe.Pointer(&der[0])),
		uintptr(len(der)),
		3, // CERT_STORE_ADD_REPLACE_EXISTING
		0,
	)
	if ret != 0 {
		return nil
	}
	return fmt.Errorf("trust: add cert: %w", err)
}

// certContext mirrors the head of a CERT_CONTEXT struct enough to read
// pbCertEncoded / cbCertEncoded; we never dereference pCertInfo from Go.
type certContext struct {
	encodingType uint32
	encodedCert  *byte
	length       uint32
	certInfo     uintptr
	store        uintptr
}

func (w windowsRootStore) enumerate(fn func(ctxPtr uintptr, der []byte) bool) error {
	var prev uintptr
	for {
		ret, _, _ := procCertEnumCertificatesInStore.Call(uintptr(w), prev)
		if ret == 0 {
			return nil
		}
		ctx := (*certContext)(unsafe.Pointer(ret)) //nolint:govet
		src := unsafe.Slice(ctx.encodedCert, ctx.length)
		buf := make([]byte, len(src))
		copy(buf, src)
		_ = fn(ret, buf)
		dup, _, _ := procCertDuplicateCertificateContext.Call(ret)
		prev = dup
	}
}

func (w windowsRootStore) fingerprintsByCN(cn string) ([]string, error) {
	var fps []string
	err := w.enumerate(func(_ uintptr, der []byte) bool {
		c, perr := x509.ParseCertificate(der)
		if perr != nil {
			return true
		}
		if c.Subject.CommonName != cn {
			return true
		}
		sum := sha1.Sum(der) //nolint:gosec
		fps = append(fps, strings.ToUpper(hex.EncodeToString(sum[:])))
		return true
	})
	return fps, err
}

func (w windowsRootStore) deleteCertsWithSerial(serial *big.Int) (bool, error) {
	var deleted bool
	var lastErr error
	err := w.enumerate(func(ctxPtr uintptr, der []byte) bool {
		c, perr := x509.ParseCertificate(der)
		if perr != nil {
			return true
		}
		if c.SerialNumber.Cmp(serial) != 0 {
			return true
		}
		ret, _, derr := procCertDeleteCertificateFromStore.Call(ctxPtr)
		if ret != 0 {
			deleted = true
			return false
		}
		lastErr = fmt.Errorf("trust: delete cert: %w", derr)
		return true
	})
	if err != nil {
		return deleted, err
	}
	return deleted, lastErr
}
