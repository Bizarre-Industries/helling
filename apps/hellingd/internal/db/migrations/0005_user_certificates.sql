-- +goose Up
-- +goose StatementBegin

-- user_certificates stores per-user Incus client certificates issued by the
-- Helling internal CA (ADR-024 + docs/spec/internal-ca.md §4.3). cert_pem
-- and private_key_pem are age-encrypted blobs; callers decrypt with the
-- host identity stored at /var/lib/helling/ca-identity.
CREATE TABLE user_certificates (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    serial_number     TEXT NOT NULL,
    cert_pem          BLOB NOT NULL,
    private_key_pem   BLOB NOT NULL,
    public_key_sha256 TEXT NOT NULL,
    issued_at         INTEGER NOT NULL,
    expires_at        INTEGER NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('active','superseded','expired')),
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- Only one active cert per user at a time (spec §4.3 UNIQUE(user_id,status)
-- rewritten as a partial index so superseded/expired rows stay distinguishable).
CREATE UNIQUE INDEX idx_user_certificates_active_unique
    ON user_certificates(user_id) WHERE status = 'active';

CREATE INDEX idx_user_certificates_serial ON user_certificates(serial_number);
CREATE INDEX idx_user_certificates_expires ON user_certificates(expires_at);

-- user_certificate_hashes holds plaintext audit metadata (no secrets).
CREATE TABLE user_certificate_hashes (
    cert_serial TEXT PRIMARY KEY,
    sha256_hash TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS user_certificate_hashes;
DROP INDEX IF EXISTS idx_user_certificates_expires;
DROP INDEX IF EXISTS idx_user_certificates_serial;
DROP INDEX IF EXISTS idx_user_certificates_active_unique;
DROP TABLE IF EXISTS user_certificates;

-- +goose StatementEnd
