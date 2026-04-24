package pki_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
)

func TestEnsureCA_BootstrapsAndReloads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	paths := pki.NewTestPaths(dir)

	first, created, err := pki.EnsureCA(paths, nil)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if !created {
		t.Fatal("expected bootstrap on empty dir")
	}
	for _, p := range []string{paths.Identity, paths.KeyAge, paths.Cert} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing artifact %s: %v", p, err)
		}
	}

	second, created, err := pki.EnsureCA(paths, nil)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if created {
		t.Fatal("second call should reload, not bootstrap")
	}
	if !bytes.Equal(first.CertPEM, second.CertPEM) {
		t.Fatal("cert mismatch on reload")
	}
	if !bytes.Equal(first.Key, second.Key) {
		t.Fatal("key mismatch on reload")
	}
}

func TestEnsureCA_RejectsTamperedKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	paths := pki.NewTestPaths(dir)
	if _, _, err := pki.EnsureCA(paths, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KeyAge, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pki.EnsureCA(paths, nil); err == nil {
		t.Fatal("expected error after tampering")
	}
}

func TestDefaultPaths_NonEmpty(t *testing.T) {
	t.Parallel()
	p := pki.DefaultPaths()
	if p.Identity == "" || p.KeyAge == "" || p.Cert == "" {
		t.Fatal("default paths must be set")
	}
}
