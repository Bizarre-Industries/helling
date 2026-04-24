package pki_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/db"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/repo/authrepo"
)

func newRenewerTestRepo(t *testing.T) *authrepo.Repo {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "renewer.db") + "?cache=shared"
	pool, err := db.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	return authrepo.New(pool)
}

func TestRenewer_RotatesExpiringCert(t *testing.T) {
	ctx := context.Background()
	repo := newRenewerTestRepo(t)
	u, err := repo.CreateUser(ctx, "alice", "admin", "")
	if err != nil {
		t.Fatal(err)
	}
	ca, err := pki.Bootstrap(nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := pki.NewIdentity()
	iss := &pki.Issuer{CA: ca, Identity: id, Repo: repo}
	if err := iss.IssueForUser(ctx, u.ID, u.Username); err != nil {
		t.Fatal(err)
	}
	original, err := repo.GetActiveUserCertificate(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Advance the renewer's clock so the row falls inside the renewal
	// threshold.
	r := &pki.Renewer{
		Issuer: iss,
		Repo:   repo,
		Now:    func() time.Time { return time.Now().Add(pki.UserCertValidity - 30*24*time.Hour) },
	}
	n, err := r.Tick(ctx)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if n != 1 {
		t.Fatalf("renewed count = %d, want 1", n)
	}
	got, err := repo.GetActiveUserCertificate(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SerialNumber == original.SerialNumber {
		t.Fatal("expected new serial after renewal")
	}
}

func TestRenewer_NoOpWhenNothingExpiring(t *testing.T) {
	ctx := context.Background()
	repo := newRenewerTestRepo(t)
	u, _ := repo.CreateUser(ctx, "bob", "admin", "")
	ca, _ := pki.Bootstrap(nil)
	id, _ := pki.NewIdentity()
	iss := &pki.Issuer{CA: ca, Identity: id, Repo: repo}
	_ = iss.IssueForUser(ctx, u.ID, u.Username)

	r := &pki.Renewer{Issuer: iss, Repo: repo}
	n, err := r.Tick(ctx)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 renewals, got %d", n)
	}
}

func TestRenewer_RejectsMissingDeps(t *testing.T) {
	var r *pki.Renewer
	if _, err := r.Tick(context.Background()); err == nil {
		t.Fatal("nil renewer should error")
	}
	if _, err := (&pki.Renewer{}).Tick(context.Background()); err == nil {
		t.Fatal("unconfigured renewer should error")
	}
}
