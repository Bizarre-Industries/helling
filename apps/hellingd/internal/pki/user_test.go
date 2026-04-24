package pki_test

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
)

func TestIssueUserCert_SignedByCA(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	ca, err := pki.Bootstrap(&pki.CAConfig{Clock: fixedClock(now)})
	if err != nil {
		t.Fatal(err)
	}
	uc, err := ca.IssueUserCert(pki.UserCertRequest{Username: "alice", UserID: "u_01"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	block, _ := pem.Decode(uc.CertPEM)
	if block == nil {
		t.Fatal("cert pem decode failed")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cert.Subject.CommonName != "alice" {
		t.Fatalf("CN=%q", cert.Subject.CommonName)
	}
	if len(cert.Subject.OrganizationalUnit) == 0 || cert.Subject.OrganizationalUnit[0] != "u_01" {
		t.Fatalf("OU=%v", cert.Subject.OrganizationalUnit)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now.Add(24 * time.Hour),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uc.ExpiresAt.Sub(uc.IssuedAt) != pki.UserCertValidity {
		t.Fatalf("validity mismatch: %v", uc.ExpiresAt.Sub(uc.IssuedAt))
	}
	if len(uc.PublicKeySHA) != 32 {
		t.Fatalf("expected 32-byte SHA, got %d", len(uc.PublicKeySHA))
	}
	if uc.SerialHex == "" {
		t.Fatal("missing serial")
	}
}

func TestIssueUserCert_RejectsBadRequest(t *testing.T) {
	t.Parallel()
	ca, _ := pki.Bootstrap(&pki.CAConfig{})
	if _, err := ca.IssueUserCert(pki.UserCertRequest{UserID: "u"}); err == nil {
		t.Fatal("expected error on missing username")
	}
	if _, err := ca.IssueUserCert(pki.UserCertRequest{Username: "alice"}); err == nil {
		t.Fatal("expected error on missing user id")
	}
}

func TestNeedsRenewal_BoundaryIsInclusive(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	exp := now.Add(pki.UserCertRenewalThreshold)
	if !pki.NeedsRenewal(exp, now) {
		t.Fatal("threshold boundary should trigger renewal")
	}
	if pki.NeedsRenewal(exp.Add(time.Second), now) {
		t.Fatal("just past threshold should not trigger")
	}
}

func TestExpired_GracePeriodHonored(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	exp := now.Add(-1 * time.Second)
	if pki.Expired(exp, now) {
		t.Fatal("within grace period should not be expired")
	}
	if !pki.Expired(exp.Add(-pki.UserCertGracePeriod), now) {
		t.Fatal("past grace period should be expired")
	}
}
