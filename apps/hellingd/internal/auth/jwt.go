package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Signer issues and verifies Ed25519 JWTs. See ADR-031 and
// docs/spec/auth.md §2.2 for the required claim set.
type Signer struct {
	priv          ed25519.PrivateKey
	pub           ed25519.PublicKey
	accessTTL     time.Duration
	refreshTTL    time.Duration
	inactivityTTL time.Duration
	issuer        string
	now           func() time.Time
}

// Claims is the Helling access-token claim body.
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Tokens is an issued access+refresh pair.
type Tokens struct {
	AccessToken     string
	AccessExpiresIn int
	RefreshToken    string
	RefreshExpires  time.Time
}

// ErrInvalidToken wraps all verification failures so callers can respond with
// a single 401 code.
var ErrInvalidToken = errors.New("auth: invalid token")

// NewSigner builds a Signer for the given Ed25519 key. The caller owns key
// lifecycle (load from disk, rotate, etc).
func NewSigner(priv ed25519.PrivateKey, issuer string, accessTTL, refreshTTL, inactivityTTL time.Duration) *Signer {
	pub, _ := priv.Public().(ed25519.PublicKey)
	return &Signer{
		priv:          priv,
		pub:           pub,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		inactivityTTL: inactivityTTL,
		issuer:        issuer,
		now:           time.Now,
	}
}

// GenerateKey produces a fresh Ed25519 keypair. Useful for tests and first-
// boot bootstrap before ADR-031 key material is persisted.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// IssueAccess mints a short-lived JWT for the given user identity.
func (s *Signer) IssueAccess(userID, username, role string) (token string, expiresInSeconds int, err error) {
	if userID == "" || username == "" || role == "" {
		return "", 0, errors.New("auth: userID/username/role required")
	}
	now := s.now()
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        randomID(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := tok.SignedString(s.priv)
	if err != nil {
		return "", 0, fmt.Errorf("auth: sign: %w", err)
	}
	return signed, int(s.accessTTL.Seconds()), nil
}

// IssueRefresh mints a random opaque refresh token. The caller must persist
// its SHA-256 hash server-side so it is revocable (see docs/spec/auth.md §2.2).
func (s *Signer) IssueRefresh() (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("auth: read refresh: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), s.now().Add(s.refreshTTL), nil
}

// Verify parses and validates an access token. Returns claims on success.
func (s *Signer) Verify(token string) (*Claims, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, ErrInvalidToken
		}
		return s.pub, nil
	}, jwt.WithIssuedAt(), jwt.WithIssuer(s.issuer))
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// AccessTTL returns the access-token lifetime this signer issues.
func (s *Signer) AccessTTL() time.Duration { return s.accessTTL }

// RefreshTTL returns the refresh-token lifetime this signer issues.
func (s *Signer) RefreshTTL() time.Duration { return s.refreshTTL }

// InactivityTTL returns the configured session inactivity timeout.
func (s *Signer) InactivityTTL() time.Duration { return s.inactivityTTL }

// Public exposes the public key so middleware in other packages can verify
// tokens without re-parsing key material.
func (s *Signer) Public() ed25519.PublicKey { return s.pub }

// SetClock lets tests replace time.Now deterministically.
func (s *Signer) SetClock(now func() time.Time) { s.now = now }

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
