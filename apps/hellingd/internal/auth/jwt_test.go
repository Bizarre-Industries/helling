package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	_, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return NewSigner(priv, "hellingd-test", 15*time.Minute, 7*24*time.Hour, 30*time.Minute)
}

func TestIssueAndVerifyAccess(t *testing.T) {
	s := newTestSigner(t)
	tok, ttl, err := s.IssueAccess("user_01", "alice", "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ttl != 900 {
		t.Errorf("ttl = %d, want 900", ttl)
	}
	claims, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "user_01" || claims.Username != "alice" || claims.Role != "admin" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if claims.Issuer != "hellingd-test" {
		t.Errorf("issuer = %q", claims.Issuer)
	}
	if claims.ID == "" {
		t.Error("jti required")
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	a := newTestSigner(t)
	_, priv, _ := GenerateKey()
	b := NewSigner(priv, "other", time.Minute, time.Hour, time.Hour)

	tok, _, _ := a.IssueAccess("u", "u", "user")
	if _, err := b.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	s := newTestSigner(t)
	s.SetClock(func() time.Time { return time.Unix(1_700_000_000, 0) })
	tok, _, _ := s.IssueAccess("u", "u", "user")
	s.SetClock(func() time.Time { return time.Unix(1_700_000_000, 0).Add(time.Hour) })
	if _, err := s.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestIssueAccessRejectsEmptyInputs(t *testing.T) {
	s := newTestSigner(t)
	if _, _, err := s.IssueAccess("", "alice", "admin"); err == nil {
		t.Fatal("expected error on empty userID")
	}
}

func TestIssueRefresh(t *testing.T) {
	s := newTestSigner(t)
	tok, expires, err := s.IssueRefresh()
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if tok == "" || strings.Contains(tok, "=") {
		t.Errorf("refresh token shape wrong: %q", tok)
	}
	if expires.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Errorf("refresh expires too soon: %v", expires)
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	s := newTestSigner(t)
	tok, _, _ := s.IssueAccess("u", "alice", "admin")
	tampered := tok[:len(tok)-4] + "AAAA"
	if _, err := s.Verify(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
