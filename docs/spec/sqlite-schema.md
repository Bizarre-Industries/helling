# SQLite Schema (v0.1)

> Status: Draft

This document defines the Helling-owned SQLite schema for v0.1.

Principles:

- SQLite stores Helling control-plane state only
- Incus and Podman remain source of truth for runtime resources
- Secrets are encrypted at rest before persistence
- Every mutable auth artifact includes timestamps for auditability

## 1. Core Tables

### 1.1 users

| Column     | Type                 | Notes                      |
| ---------- | -------------------- | -------------------------- |
| id         | TEXT PRIMARY KEY     | UUID                       |
| username   | TEXT UNIQUE NOT NULL | Login identity             |
| role       | TEXT NOT NULL        | `admin`, `user`, `auditor` |
| status     | TEXT NOT NULL        | `active`, `disabled`       |
| created_at | INTEGER NOT NULL     | Unix epoch seconds         |
| updated_at | INTEGER NOT NULL     | Unix epoch seconds         |

### 1.2 sessions

| Column             | Type             | Notes                        |
| ------------------ | ---------------- | ---------------------------- |
| id                 | TEXT PRIMARY KEY | UUID                         |
| user_id            | TEXT NOT NULL    | FK to users.id               |
| refresh_token_hash | TEXT NOT NULL    | SHA-256 digest               |
| user_agent         | TEXT             | Client fingerprint for audit |
| ip_address         | TEXT             | Last known source IP         |
| expires_at         | INTEGER NOT NULL | Unix epoch seconds           |
| revoked_at         | INTEGER          | Null when active             |
| created_at         | INTEGER NOT NULL | Unix epoch seconds           |

Index:

- `idx_sessions_user_id` on `(user_id)`
- `idx_sessions_expires_at` on `(expires_at)`

### 1.3 api_tokens

| Column       | Type             | Notes                    |
| ------------ | ---------------- | ------------------------ |
| id           | TEXT PRIMARY KEY | UUID                     |
| user_id      | TEXT NOT NULL    | FK to users.id           |
| name         | TEXT NOT NULL    | Human label              |
| token_hash   | TEXT NOT NULL    | SHA-256 digest           |
| scope        | TEXT NOT NULL    | `read`, `write`, `admin` |
| last_used_at | INTEGER          | Unix epoch seconds       |
| expires_at   | INTEGER NOT NULL | Unix epoch seconds       |
| revoked_at   | INTEGER          | Null when active         |
| created_at   | INTEGER NOT NULL | Unix epoch seconds       |

Index:

- `idx_api_tokens_user_id` on `(user_id)`

## 2. MFA Tables

### 2.1 totp_secrets

| Column           | Type             | Notes                  |
| ---------------- | ---------------- | ---------------------- |
| user_id          | TEXT PRIMARY KEY | FK to users.id         |
| encrypted_secret | BLOB NOT NULL    | Encrypted with app key |
| enabled          | INTEGER NOT NULL | 0 or 1                 |
| created_at       | INTEGER NOT NULL | Unix epoch seconds     |
| updated_at       | INTEGER NOT NULL | Unix epoch seconds     |

### 2.2 recovery_codes

| Column     | Type             | Notes              |
| ---------- | ---------------- | ------------------ |
| id         | TEXT PRIMARY KEY | UUID               |
| user_id    | TEXT NOT NULL    | FK to users.id     |
| code_hash  | TEXT NOT NULL    | argon2id hash      |
| used_at    | INTEGER          | Null when unused   |
| created_at | INTEGER NOT NULL | Unix epoch seconds |

Index:

- `idx_recovery_codes_user_id` on `(user_id)`

## 3. Incus Certificate Identity Tables

### 3.1 incus_user_certs

| Column            | Type                 | Notes                        |
| ----------------- | -------------------- | ---------------------------- |
| user_id           | TEXT PRIMARY KEY     | FK to users.id               |
| cert_pem          | TEXT NOT NULL        | Public certificate           |
| encrypted_key_pem | BLOB NOT NULL        | Encrypted private key        |
| fingerprint       | TEXT UNIQUE NOT NULL | Incus trust fingerprint      |
| restricted        | INTEGER NOT NULL     | Must be 1 for user certs     |
| project_scope     | TEXT NOT NULL        | Assigned project/limit scope |
| expires_at        | INTEGER NOT NULL     | Unix epoch seconds           |
| revoked_at        | INTEGER              | Null when active             |
| created_at        | INTEGER NOT NULL     | Unix epoch seconds           |
| updated_at        | INTEGER NOT NULL     | Unix epoch seconds           |

Index:

- `idx_incus_user_certs_fingerprint` on `(fingerprint)`
- `idx_incus_user_certs_expires_at` on `(expires_at)`

## 4. Audit Anchors

### 4.1 auth_events

| Column        | Type             | Notes                                                     |
| ------------- | ---------------- | --------------------------------------------------------- |
| id            | TEXT PRIMARY KEY | UUID                                                      |
| user_id       | TEXT             | Nullable for failed pre-auth events                       |
| event_type    | TEXT NOT NULL    | `login_ok`, `login_fail`, `token_revoked`, `cert_rotated` |
| source_ip     | TEXT             | Request source                                            |
| user_agent    | TEXT             | Request user agent                                        |
| metadata_json | TEXT             | Compact structured context                                |
| created_at    | INTEGER NOT NULL | Unix epoch seconds                                        |

Index:

- `idx_auth_events_user_id` on `(user_id)`
- `idx_auth_events_created_at` on `(created_at)`

## 5. Constraints and Migrations

- Foreign keys are enabled with `PRAGMA foreign_keys = ON`
- Destructive migrations require backup creation before apply
- New columns must be backward-compatible for one release window
- Secret-bearing columns are never returned by public API responses
