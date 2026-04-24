package authrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UserCertStatus enumerates the lifecycle states a user certificate row may
// occupy per docs/spec/internal-ca.md §4.3.
type UserCertStatus string

// Lifecycle statuses for user_certificates.status.
const (
	// CertStatusActive marks the currently valid cert for a user.
	CertStatusActive UserCertStatus = "active"
	// CertStatusSuperseded marks a cert replaced during renewal but still
	// within the 60-day dual-sign overlap per spec §4.1.
	CertStatusSuperseded UserCertStatus = "superseded"
	// CertStatusExpired marks a cert beyond grace period; rejected at proxy.
	CertStatusExpired UserCertStatus = "expired"
)

// UserCertificate mirrors the user_certificates row.
type UserCertificate struct {
	ID              string
	UserID          string
	SerialNumber    string
	CertPEM         []byte // age-encrypted
	PrivateKeyPEM   []byte // age-encrypted
	PublicKeySHA256 string
	IssuedAt        int64
	ExpiresAt       int64
	Status          UserCertStatus
	CreatedAt       int64
	UpdatedAt       int64
}

// CreateUserCertificateInput is the payload for InsertUserCertificate.
type CreateUserCertificateInput struct {
	UserID                 string
	SerialNumber           string
	CertPEMEncrypted       []byte
	PrivateKeyPEMEncrypted []byte
	PublicKeySHA256        string
	IssuedAt               time.Time
	ExpiresAt              time.Time
}

// InsertUserCertificate inserts a fresh active certificate and records an
// audit-only hash row. Caller is responsible for age-encrypting PEM blobs.
func (r *Repo) InsertUserCertificate(ctx context.Context, in *CreateUserCertificateInput) (UserCertificate, error) {
	if in == nil {
		return UserCertificate{}, errors.New("authrepo: InsertUserCertificate: nil input")
	}
	if in.UserID == "" || in.SerialNumber == "" {
		return UserCertificate{}, errors.New("authrepo: InsertUserCertificate: user_id and serial required")
	}
	if len(in.CertPEMEncrypted) == 0 || len(in.PrivateKeyPEMEncrypted) == 0 {
		return UserCertificate{}, errors.New("authrepo: InsertUserCertificate: encrypted blobs required")
	}
	now := r.now().Unix()
	id := uuid.NewString()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserCertificate{}, fmt.Errorf("authrepo: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertCert = `INSERT INTO user_certificates
	    (id, user_id, serial_number, cert_pem, private_key_pem, public_key_sha256,
	     issued_at, expires_at, status, created_at, updated_at)
	    VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`
	if _, err := tx.ExecContext(ctx, insertCert,
		id, in.UserID, in.SerialNumber, in.CertPEMEncrypted, in.PrivateKeyPEMEncrypted,
		in.PublicKeySHA256, in.IssuedAt.Unix(), in.ExpiresAt.Unix(), now, now,
	); err != nil {
		return UserCertificate{}, fmt.Errorf("authrepo: insert user cert: %w", err)
	}
	const insertHash = `INSERT INTO user_certificate_hashes (cert_serial, sha256_hash, created_at) VALUES (?, ?, ?)`
	if _, err := tx.ExecContext(ctx, insertHash, in.SerialNumber, in.PublicKeySHA256, now); err != nil {
		return UserCertificate{}, fmt.Errorf("authrepo: insert cert hash: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return UserCertificate{}, fmt.Errorf("authrepo: commit: %w", err)
	}
	return UserCertificate{
		ID: id, UserID: in.UserID, SerialNumber: in.SerialNumber,
		CertPEM: in.CertPEMEncrypted, PrivateKeyPEM: in.PrivateKeyPEMEncrypted,
		PublicKeySHA256: in.PublicKeySHA256,
		IssuedAt:        in.IssuedAt.Unix(), ExpiresAt: in.ExpiresAt.Unix(),
		Status: CertStatusActive, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// GetActiveUserCertificate returns the single active cert for a user, or
// ErrNotFound when none exists.
func (r *Repo) GetActiveUserCertificate(ctx context.Context, userID string) (UserCertificate, error) {
	const q = `SELECT id, user_id, serial_number, cert_pem, private_key_pem, public_key_sha256,
	                  issued_at, expires_at, status, created_at, updated_at
	           FROM user_certificates
	           WHERE user_id = ? AND status = 'active'
	           LIMIT 1`
	return scanUserCertificate(r.db.QueryRowContext(ctx, q, userID).Scan)
}

// ListExpiringUserCertificates returns active certs whose expires_at is at
// or before cutoff, oldest first. Used by the renewal worker.
func (r *Repo) ListExpiringUserCertificates(ctx context.Context, cutoff time.Time, limit int) ([]UserCertificate, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `SELECT id, user_id, serial_number, cert_pem, private_key_pem, public_key_sha256,
	                  issued_at, expires_at, status, created_at, updated_at
	           FROM user_certificates
	           WHERE status = 'active' AND expires_at <= ?
	           ORDER BY expires_at ASC, id ASC
	           LIMIT ?`
	rows, err := r.db.QueryContext(ctx, q, cutoff.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("authrepo: list expiring: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]UserCertificate, 0, limit)
	for rows.Next() {
		uc, err := scanUserCertificate(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, uc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authrepo: rows expiring: %w", err)
	}
	return out, nil
}

// SupersedeUserCertificate marks a cert as 'superseded'. Used during renewal
// before inserting the replacement, so the partial-unique active index stays
// satisfied. Returns ErrNotFound when no such row.
func (r *Repo) SupersedeUserCertificate(ctx context.Context, id string) error {
	return r.setUserCertificateStatus(ctx, id, CertStatusSuperseded)
}

// ExpireUserCertificate marks a cert as 'expired'. Called by the renewal
// worker after the 60-day dual-sign overlap ends.
func (r *Repo) ExpireUserCertificate(ctx context.Context, id string) error {
	return r.setUserCertificateStatus(ctx, id, CertStatusExpired)
}

func (r *Repo) setUserCertificateStatus(ctx context.Context, id string, status UserCertStatus) error {
	const q = `UPDATE user_certificates SET status = ?, updated_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(status), r.now().Unix(), id)
	if err != nil {
		return fmt.Errorf("authrepo: set cert status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanUserCertificate(scan func(dest ...any) error) (UserCertificate, error) {
	var uc UserCertificate
	var status string
	err := scan(&uc.ID, &uc.UserID, &uc.SerialNumber, &uc.CertPEM, &uc.PrivateKeyPEM,
		&uc.PublicKeySHA256, &uc.IssuedAt, &uc.ExpiresAt, &status,
		&uc.CreatedAt, &uc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UserCertificate{}, ErrNotFound
	}
	if err != nil {
		return UserCertificate{}, fmt.Errorf("authrepo: scan user cert: %w", err)
	}
	uc.Status = UserCertStatus(status)
	return uc, nil
}
