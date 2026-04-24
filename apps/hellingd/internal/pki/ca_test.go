package pki_test

import (
	"bytes"
	"crypto/x509"
	"testing"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestBootstrap_SelfSignedEd25519(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	ca, err := pki.Bootstrap(&pki.CAConfig{Clock: fixedClock(now)})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if ca.Cert == nil || ca.Key == nil {
		t.Fatal("missing cert or key")
	}
	if !ca.Cert.IsCA {
		t.Fatal("cert is not CA")
	}
	if got := ca.Cert.PublicKeyAlgorithm; got != x509.Ed25519 {
		t.Fatalf("unexpected pubkey algo %v", got)
	}
	if ca.Cert.NotBefore != now {
		t.Fatalf("notBefore=%v want %v", ca.Cert.NotBefore, now)
	}
	if ca.Cert.NotAfter.Sub(now) != pki.CACertValidity {
		t.Fatalf("validity=%v want %v", ca.Cert.NotAfter.Sub(now), pki.CACertValidity)
	}
	if err := ca.Cert.CheckSignatureFrom(ca.Cert); err != nil {
		t.Fatalf("self-sig: %v", err)
	}
}

func TestLoad_RoundTripsKey(t *testing.T) {
	t.Parallel()
	ca, err := pki.Bootstrap(&pki.CAConfig{})
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := ca.MarshalKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := pki.Load(keyPEM, ca.CertPEM, nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal(loaded.CertPEM, ca.CertPEM) {
		t.Fatal("cert mismatch after reload")
	}
	if !bytes.Equal(loaded.Key, ca.Key) {
		t.Fatal("key mismatch after reload")
	}
}

func TestLoad_RejectsGarbage(t *testing.T) {
	t.Parallel()
	if _, err := pki.Load([]byte("not-pem"), []byte("still-not-pem"), nil); err == nil {
		t.Fatal("expected error on garbage input")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()
	id, err := pki.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("top-secret-ca-key-pem")
	ct, err := pki.EncryptWithIdentity(id, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	pt, err := pki.DecryptWithIdentity(id, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("round-trip mismatch: got %q want %q", pt, plaintext)
	}
}

func TestDecrypt_WrongIdentityFails(t *testing.T) {
	t.Parallel()
	a, _ := pki.NewIdentity()
	b, _ := pki.NewIdentity()
	ct, _ := pki.EncryptWithIdentity(a, []byte("hello"))
	if _, err := pki.DecryptWithIdentity(b, ct); err == nil {
		t.Fatal("expected decrypt failure with wrong identity")
	}
}

func TestEncrypt_InvalidIdentity(t *testing.T) {
	t.Parallel()
	if _, err := pki.EncryptWithIdentity("not-an-identity", []byte("x")); err == nil {
		t.Fatal("expected error on bad identity")
	}
}
