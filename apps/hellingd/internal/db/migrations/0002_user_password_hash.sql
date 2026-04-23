-- +goose Up
-- +goose StatementBegin

-- Adds local argon2id password hash column to users, for bootstrap/local
-- accounts created via authSetup before PAM is wired in. The v0.1 auth spec
-- (docs/spec/auth.md §2.1) still prefers PAM for OS users; this column is
-- NULL for PAM-backed users and populated only for Helling-managed accounts.

ALTER TABLE users ADD COLUMN password_hash TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite supports DROP COLUMN from 3.35. modernc.org/sqlite ships 3.45+.
ALTER TABLE users DROP COLUMN password_hash;

-- +goose StatementEnd
