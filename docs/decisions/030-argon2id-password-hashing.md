# ADR-030: Argon2id for password and recovery-code hashing

> Status: Accepted (2026-04-19)

## Context

Security guidance in the repo currently allows multiple hashing options and still contains bcrypt-era constraints. Helling v0.1 needs one normative algorithm across auth paths so implementation and documentation cannot drift.

## Decision

Use argon2id (RFC 9106) as the only supported password hashing algorithm for Helling-managed secrets.

Applies to:

- Password hashes stored by Helling-managed auth flows
- TOTP recovery code hashes
- Any future local secret-verification values in auth scope

Implementation baseline (subject to environment tuning):

- Memory cost: 64 MiB
- Iterations: 3
- Parallelism: 1
- Salt: 16 bytes minimum, cryptographically random
- Output length: 32 bytes minimum

bcrypt is not used for new hashes in v0.1.

## Consequences

- Removes ambiguity from docs and implementation
- Eliminates bcrypt-specific limits from normative guidance
- Requires explicit parameter versioning to support future tuning
- Existing bcrypt records (if encountered in migration contexts) must be rehashed to argon2id after successful verification
