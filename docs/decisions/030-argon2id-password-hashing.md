# ADR-030: Argon2id for password and recovery-code hashing

> Status: Accepted (2026-04-19)

## Context

Security guidance in the repo currently allows multiple hashing options and still contains bcrypt-era constraints. Helling v0.1 needs one normative algorithm across auth paths so implementation and documentation cannot drift.

## Decision

Use argon2id (RFC 9106, OWASP recommendation for password hashing) as the only supported password hashing algorithm for Helling-managed secrets.

Applies to:

- Password hashes stored by Helling-managed auth flows
- TOTP recovery code hashes
- Any future local secret-verification values in auth scope

Implementation baseline per OWASP password hashing selection:

- Memory cost (m): 64 MiB (per RFC 9106 Medium category)
- Time cost (t): 3 iterations
- Parallelism (p): 1 thread
- Salt: 16 bytes minimum, cryptographically random
- Output length: 32 bytes minimum

bcrypt is not used for new hashes in v0.1.

## Consequences

- Removes ambiguity from docs and implementation
- Eliminates bcrypt-specific limits from normative guidance
- Aligns with OWASP password hashing recommendations (RFC 9106)
- Requires explicit parameter versioning to support future tuning per RFC 9106 PHC string format
- Existing bcrypt records (if encountered in migration contexts) must be rehashed to argon2id after successful verification
