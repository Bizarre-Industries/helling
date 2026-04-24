package authrepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/repo/authrepo"
)

func mustInsertCert(t *testing.T, r *authrepo.Repo, userID, serial string) authrepo.UserCertificate {
	t.Helper()
	issued := time.Unix(1700000000, 0).UTC()
	uc, err := r.InsertUserCertificate(context.Background(), &authrepo.CreateUserCertificateInput{
		UserID:                 userID,
		SerialNumber:           serial,
		CertPEMEncrypted:       []byte("enc-cert-" + serial),
		PrivateKeyPEMEncrypted: []byte("enc-key-" + serial),
		PublicKeySHA256:        "0f00" + serial,
		IssuedAt:               issued,
		ExpiresAt:              issued.Add(90 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("insert cert: %v", err)
	}
	return uc
}

func TestUserCertificate_InsertActiveAndFetch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u, err := r.CreateUser(ctx, "alice", "admin", "")
	if err != nil {
		t.Fatal(err)
	}
	uc := mustInsertCert(t, r, u.ID, "aaaa")
	got, err := r.GetActiveUserCertificate(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != uc.ID || got.Status != authrepo.CertStatusActive {
		t.Fatalf("unexpected: %+v", got)
	}
	if string(got.CertPEM) != "enc-cert-aaaa" || string(got.PrivateKeyPEM) != "enc-key-aaaa" {
		t.Fatal("encrypted blobs not round-tripped")
	}
}

func TestUserCertificate_ActivePartialUniqueIndex(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u, _ := r.CreateUser(ctx, "bob", "admin", "")
	mustInsertCert(t, r, u.ID, "bbbb")
	_, err := r.InsertUserCertificate(ctx, &authrepo.CreateUserCertificateInput{
		UserID:                 u.ID,
		SerialNumber:           "cccc",
		CertPEMEncrypted:       []byte("x"),
		PrivateKeyPEMEncrypted: []byte("y"),
		PublicKeySHA256:        "beef",
		IssuedAt:               time.Now(),
		ExpiresAt:              time.Now().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected unique-index violation for second active cert")
	}
}

func TestUserCertificate_SupersedeAndExpire(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u, _ := r.CreateUser(ctx, "carol", "admin", "")
	uc := mustInsertCert(t, r, u.ID, "dddd")
	if err := r.SupersedeUserCertificate(ctx, uc.ID); err != nil {
		t.Fatalf("supersede: %v", err)
	}
	mustInsertCert(t, r, u.ID, "eeee")
	if err := r.ExpireUserCertificate(ctx, uc.ID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if err := r.ExpireUserCertificate(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected ErrNotFound on unknown id")
	}
}

func TestUserCertificate_ListExpiring(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u, _ := r.CreateUser(ctx, "dan", "admin", "")
	uc := mustInsertCert(t, r, u.ID, "1111")
	cutoff := time.Unix(uc.ExpiresAt, 0).UTC().Add(1 * time.Second)
	got, err := r.ListExpiringUserCertificates(ctx, cutoff, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 expiring, got %d", len(got))
	}
}

func TestUserCertificate_RejectsInvalidInput(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.InsertUserCertificate(ctx, &authrepo.CreateUserCertificateInput{}); err == nil {
		t.Fatal("expected error on empty input")
	}
	if _, err := r.InsertUserCertificate(ctx, &authrepo.CreateUserCertificateInput{
		UserID: "u", SerialNumber: "s",
	}); err == nil {
		t.Fatal("expected error on missing blobs")
	}
}

func TestUserCertificate_NoActiveReturnsNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	u, _ := r.CreateUser(ctx, "erin", "admin", "")
	if _, err := r.GetActiveUserCertificate(ctx, u.ID); err == nil {
		t.Fatal("expected ErrNotFound when no cert exists")
	}
}
