// Package pki implements the Helling internal CA and per-user Incus client
// certificates per ADR-024 and docs/spec/internal-ca.md. This package owns
// CA-keypair generation, age-encryption of persisted material, and user-cert
// issuance. Database storage is handled by authrepo; wiring to the Incus
// trust store is handled by the proxy layer.
package pki

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"filippo.io/age"
)

// CACertValidity is the lifetime of the self-signed CA certificate.
// Per docs/spec/internal-ca.md §3.1: 5 years from generation.
const CACertValidity = 5 * 365 * 24 * time.Hour

// UserCertValidity is the lifetime of an issued user certificate.
// Per docs/spec/internal-ca.md §4.1: 90 days.
const UserCertValidity = 90 * 24 * time.Hour

// UserCertRenewalThreshold defines when automatic renewal should trigger.
// Per docs/spec/internal-ca.md §4.1: renew at 60 days remaining.
const UserCertRenewalThreshold = 60 * 24 * time.Hour

// UserCertGracePeriod is the window after technical expiry where requests
// still succeed so operators have time to refresh. Per spec §4.1.
const UserCertGracePeriod = 10 * 24 * time.Hour

// DefaultCASubject is the X.500 subject applied to the CA certificate
// when callers do not override via CAConfig.Subject.
var DefaultCASubject = pkix.Name{
	CommonName:   "Helling CA",
	Organization: []string{"Bizarre Industries"},
}

// CA carries an active CA keypair + certificate in memory. It is produced
// by Bootstrap or Load and must never be serialized unencrypted.
type CA struct {
	Key     ed25519.PrivateKey
	Cert    *x509.Certificate
	CertPEM []byte
	clock   func() time.Time
	rand    io.Reader
}

// Now returns the CA's configured clock (time.Now in production; overridable
// in tests).
func (c *CA) Now() time.Time { return c.clock() }

// CAConfig holds options accepted by Bootstrap.
type CAConfig struct {
	Subject    pkix.Name
	Validity   time.Duration // default CACertValidity
	Clock      func() time.Time
	RandReader io.Reader
}

// Bootstrap generates a fresh Ed25519 CA keypair and self-signed certificate.
// Called once at first hellingd startup; Load is used on subsequent boots.
func Bootstrap(cfg *CAConfig) (*CA, error) {
	if cfg == nil {
		cfg = &CAConfig{}
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	r := cfg.RandReader
	if r == nil {
		r = rand.Reader
	}
	subj := cfg.Subject
	if subj.CommonName == "" {
		subj = DefaultCASubject
	}
	validity := cfg.Validity
	if validity == 0 {
		validity = CACertValidity
	}

	pub, priv, err := ed25519.GenerateKey(r)
	if err != nil {
		return nil, fmt.Errorf("pki: generate ca keypair: %w", err)
	}
	serial, err := randomSerial(r)
	if err != nil {
		return nil, err
	}
	now := clock().UTC().Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subj,
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(r, tmpl, tmpl, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("pki: sign ca cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: parse ca cert: %w", err)
	}
	return &CA{
		Key:     priv,
		Cert:    cert,
		CertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		clock:   clock,
		rand:    r,
	}, nil
}

// Load reconstructs a CA from a PEM-encoded PKCS#8 private key plus
// PEM-encoded certificate. Used on hellingd startup after Bootstrap.
func Load(keyPEM, certPEM []byte, clock func() time.Time) (*CA, error) {
	if clock == nil {
		clock = time.Now
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("pki: ca key pem decode failed")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse ca key: %w", err)
	}
	key, ok := keyAny.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("pki: ca key is not ed25519")
	}
	cblock, _ := pem.Decode(certPEM)
	if cblock == nil {
		return nil, errors.New("pki: ca cert pem decode failed")
	}
	cert, err := x509.ParseCertificate(cblock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse ca cert: %w", err)
	}
	return &CA{
		Key:     key,
		Cert:    cert,
		CertPEM: certPEM,
		clock:   clock,
		rand:    rand.Reader,
	}, nil
}

// MarshalKeyPEM returns a PKCS#8-wrapped PEM of the CA private key. Callers
// MUST encrypt this blob with age before persisting it.
func (c *CA) MarshalKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(c.Key)
	if err != nil {
		return nil, fmt.Errorf("pki: marshal ca key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func randomSerial(r io.Reader) (*big.Int, error) {
	// RFC 5280 §4.1.2.2: serial up to 20 octets (159 bits usable after sign bit).
	maxSerial := new(big.Int).Lsh(big.NewInt(1), 159)
	n, err := rand.Int(r, maxSerial)
	if err != nil {
		return nil, fmt.Errorf("pki: random serial: %w", err)
	}
	return n, nil
}

// EncryptWithIdentity wraps age-encryption for CA or user-cert blobs. id must
// be an age X25519 identity string (the on-host `ca-identity` file contents).
func EncryptWithIdentity(id string, plaintext []byte) ([]byte, error) {
	recipient, err := ageRecipientFromIdentity(id)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("pki: age encrypt: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("pki: age write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("pki: age close: %w", err)
	}
	return buf.Bytes(), nil
}

// DecryptWithIdentity is the inverse of EncryptWithIdentity.
func DecryptWithIdentity(id string, ciphertext []byte) ([]byte, error) {
	ident, err := ageIdentityFromString(id)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), ident)
	if err != nil {
		return nil, fmt.Errorf("pki: age decrypt: %w", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("pki: age read: %w", err)
	}
	return out, nil
}

// NewIdentity mints a fresh age X25519 identity (private key + recipient
// derivation). Return value is stored plaintext at
// /var/lib/helling/ca-identity per spec §2.
func NewIdentity() (string, error) {
	i, err := age.GenerateX25519Identity()
	if err != nil {
		return "", fmt.Errorf("pki: generate age identity: %w", err)
	}
	return i.String(), nil
}

func ageIdentityFromString(s string) (*age.X25519Identity, error) {
	return age.ParseX25519Identity(strings.TrimSpace(s))
}

func ageRecipientFromIdentity(id string) (*age.X25519Recipient, error) {
	i, err := ageIdentityFromString(id)
	if err != nil {
		return nil, err
	}
	return i.Recipient(), nil
}
