package pki

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// UserCert bundles an issued user certificate's PEM artifacts plus metadata.
type UserCert struct {
	SerialHex      string
	CertPEM        []byte
	PrivateKeyPEM  []byte
	IssuedAt       time.Time
	ExpiresAt      time.Time
	Subject        pkix.Name
	PublicKeySHA   []byte // 32-byte SHA-256 of DER SubjectPublicKeyInfo.
	CertDER        []byte
	PrivateKeyPKCS []byte
}

// UserCertRequest configures a single issuance.
type UserCertRequest struct {
	Username  string
	UserID    string
	Validity  time.Duration // default UserCertValidity
	NotBefore time.Time     // default ca.Now()
}

// IssueUserCert signs a fresh Ed25519 keypair for a user against this CA.
// Returned PEM artifacts are plaintext; callers MUST age-encrypt before
// persistence per docs/spec/internal-ca.md §4.3.
func (c *CA) IssueUserCert(req UserCertRequest) (*UserCert, error) {
	if req.Username == "" {
		return nil, errors.New("pki: IssueUserCert: username required")
	}
	if req.UserID == "" {
		return nil, errors.New("pki: IssueUserCert: user id required")
	}
	validity := req.Validity
	if validity == 0 {
		validity = UserCertValidity
	}
	notBefore := req.NotBefore
	if notBefore.IsZero() {
		notBefore = c.clock()
	}
	notBefore = notBefore.UTC().Truncate(time.Second)

	pub, priv, err := ed25519.GenerateKey(c.rand)
	if err != nil {
		return nil, fmt.Errorf("pki: generate user keypair: %w", err)
	}
	serial, err := randomSerial(c.rand)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         req.Username,
			Organization:       []string{"Helling Users"},
			OrganizationalUnit: []string{req.UserID},
		},
		NotBefore:   notBefore,
		NotAfter:    notBefore.Add(validity),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(c.rand, tmpl, c.Cert, pub, c.Key)
	if err != nil {
		return nil, fmt.Errorf("pki: sign user cert: %w", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: parse user cert: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("pki: marshal user key: %w", err)
	}
	spki := sha256.Sum256(parsed.RawSubjectPublicKeyInfo)
	return &UserCert{
		SerialHex:      fmt.Sprintf("%x", serial),
		CertPEM:        pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		PrivateKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		IssuedAt:       notBefore,
		ExpiresAt:      notBefore.Add(validity),
		Subject:        tmpl.Subject,
		PublicKeySHA:   spki[:],
		CertDER:        der,
		PrivateKeyPKCS: keyDER,
	}, nil
}

// NeedsRenewal reports whether a cert expiring at exp should be rotated
// given the current time. Uses UserCertRenewalThreshold (60 days).
func NeedsRenewal(exp, now time.Time) bool {
	return exp.Sub(now) <= UserCertRenewalThreshold
}

// Expired reports whether a cert expiring at exp is past hard expiry plus
// the grace period. Requests after this point must be rejected.
func Expired(exp, now time.Time) bool {
	return now.After(exp.Add(UserCertGracePeriod))
}
