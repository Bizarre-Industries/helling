package pki

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Paths captures the on-host filesystem layout from docs/spec/internal-ca.md §2.
// Identity is the age X25519 secret stored plaintext (controls the encrypted
// CA key); KeyAge holds the age-encrypted CA private key; Cert holds the
// plaintext CA certificate PEM.
type Paths struct {
	Identity string
	KeyAge   string
	Cert     string
}

// DefaultPaths returns the production layout under /etc/helling and
// /var/lib/helling. Tests use NewTestPaths to point at a TempDir.
func DefaultPaths() Paths {
	return Paths{
		Identity: "/var/lib/helling/ca-identity",
		KeyAge:   "/etc/helling/ca.key.age",
		Cert:     "/etc/helling/ca.crt",
	}
}

// NewTestPaths returns paths under root suitable for *_test.go usage.
func NewTestPaths(root string) Paths {
	return Paths{
		Identity: filepath.Join(root, "ca-identity"),
		KeyAge:   filepath.Join(root, "ca.key.age"),
		Cert:     filepath.Join(root, "ca.crt"),
	}
}

// EnsureCA loads an existing CA from p; if absent it bootstraps a new one,
// writing all three artifacts. Returns the loaded/created CA plus a flag
// indicating whether bootstrap occurred (callers may want to log that).
func EnsureCA(p Paths, logger *slog.Logger) (*CA, bool, error) {
	id, err := loadOrCreateIdentity(p.Identity)
	if err != nil {
		return nil, false, err
	}
	if exists(p.KeyAge) && exists(p.Cert) {
		ca, err := loadCAFromDisk(id, p)
		if err != nil {
			return nil, false, fmt.Errorf("pki: load ca: %w", err)
		}
		return ca, false, nil
	}
	ca, err := Bootstrap(nil)
	if err != nil {
		return nil, false, err
	}
	if err := writeBootstrapped(id, ca, p); err != nil {
		return nil, false, err
	}
	if logger != nil {
		logger.Info("internal ca bootstrapped",
			slog.String("identity", p.Identity),
			slog.String("cert", p.Cert))
	}
	return ca, true, nil
}

func loadOrCreateIdentity(path string) (string, error) {
	if exists(path) {
		raw, err := os.ReadFile(path) //nolint:gosec // operator-controlled host path
		if err != nil {
			return "", fmt.Errorf("pki: read identity: %w", err)
		}
		return string(raw), nil
	}
	id, err := NewIdentity()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("pki: mkdir identity: %w", err)
	}
	if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
		return "", fmt.Errorf("pki: write identity: %w", err)
	}
	return id, nil
}

func loadCAFromDisk(id string, p Paths) (*CA, error) {
	encKey, err := os.ReadFile(p.KeyAge)
	if err != nil {
		return nil, fmt.Errorf("pki: read ca.key.age: %w", err)
	}
	keyPEM, err := DecryptWithIdentity(id, encKey)
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(p.Cert)
	if err != nil {
		return nil, fmt.Errorf("pki: read ca.crt: %w", err)
	}
	return Load(keyPEM, certPEM, nil)
}

func writeBootstrapped(id string, ca *CA, p Paths) error {
	keyPEM, err := ca.MarshalKeyPEM()
	if err != nil {
		return err
	}
	encKey, err := EncryptWithIdentity(id, keyPEM)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.KeyAge), 0o750); err != nil {
		return fmt.Errorf("pki: mkdir ca.key.age: %w", err)
	}
	if err := os.WriteFile(p.KeyAge, encKey, 0o600); err != nil {
		return fmt.Errorf("pki: write ca.key.age: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.Cert), 0o750); err != nil {
		return fmt.Errorf("pki: mkdir ca.crt: %w", err)
	}
	if err := os.WriteFile(p.Cert, ca.CertPEM, 0o644); err != nil { //nolint:gosec // CA cert is public material
		return fmt.Errorf("pki: write ca.crt: %w", err)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}
