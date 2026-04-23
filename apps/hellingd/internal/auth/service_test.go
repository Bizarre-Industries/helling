package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/auth"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/db"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/repo/authrepo"
)

func newTestService(t *testing.T) *auth.Service {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "auth.db") + "?cache=shared"
	pool, err := db.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	_, priv, err := auth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer := auth.NewSigner(priv, "hellingd-test", 15*time.Minute, 7*24*time.Hour, 30*time.Minute)
	return auth.NewService(authrepo.New(pool), signer, auth.Argon2idParams{})
}

func TestService_SetupThenLoginAndRefresh(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	required, err := s.SetupRequired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !required {
		t.Fatal("fresh DB should require setup")
	}

	ident, err := s.Setup(ctx, "admin", "correct-horse-battery-staple", "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if ident.UserID == "" || ident.AccessToken == "" || ident.RefreshToken == "" {
		t.Fatalf("setup identity missing fields: %+v", ident)
	}
	if ident.Role != "admin" {
		t.Errorf("role = %q, want admin", ident.Role)
	}

	if _, err := s.Setup(ctx, "admin", "x", "", ""); !errors.Is(err, auth.ErrSetupNotRequired) {
		t.Fatalf("expected ErrSetupNotRequired, got %v", err)
	}

	login, err := s.Login(ctx, "admin", "correct-horse-battery-staple", "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatal("login tokens missing")
	}

	if _, err := s.Login(ctx, "admin", "nope", "", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if _, err := s.Login(ctx, "ghost", "x", "", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	refreshed, err := s.Refresh(ctx, login.RefreshToken, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	if _, err := s.Refresh(ctx, login.RefreshToken, "", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("old refresh should be revoked, got %v", err)
	}
}

func TestService_LogoutRevokesRefresh(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	ident, err := s.Setup(ctx, "admin", "password1234", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, ident.RefreshToken, "", ""); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := s.Refresh(ctx, ident.RefreshToken, "", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials after logout, got %v", err)
	}
}

func TestService_VerifyIssuedAccessToken(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	ident, err := s.Setup(ctx, "admin", "password1234", "", "")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.Signer().Verify(ident.AccessToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != ident.UserID || claims.Username != "admin" || claims.Role != "admin" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestService_SetupRejectsEmptyCredentials(t *testing.T) {
	s := newTestService(t)
	if _, err := s.Setup(context.Background(), "", "pass", "", ""); err == nil {
		t.Fatal("expected error on empty username")
	}
	if _, err := s.Setup(context.Background(), "admin", "", "", ""); err == nil {
		t.Fatal("expected error on empty password")
	}
}
