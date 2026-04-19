# OpenAPI Pipeline

**Date:** 2026-04-20
**Supersedes:** 2026-04-15 docs-first + orval server/client generation flow

---

## The Pipeline

Helling now uses a code-first contract flow for Helling-owned API routes.

```text
Go input/output structs + validation tags + operation metadata
        |
        v
Huma v2 operations mounted via humago on net/http ServeMux
(Helling-owned /api/v1/* only)
        |
        v
Generate api/openapi.yaml from registered operations
        |
        v
Commit generated contract artifact for review
        |
   +----+-------------------------+
   |                              |
   v                              v
oapi-codegen (CLI client)     hey-api/openapi-ts (WebUI SDK + Query)
   |                              |
apps/helling-cli/internal      web/src/api/generated
```

Proxy pass-through paths stay plain handlers:

- /api/incus/\*
- /api/podman/\*

These routes bypass Huma to preserve ADR-014 and ADR-015 behavior.

---

## Scope and Invariants

- Huma manages approximately 34 Helling-owned endpoints under /api/v1/\*.
- ServeMux remains the top-level router (ADR-040 preserved via humago adapter).
- URI major versioning remains /api/v1 (ADR-041 preserved).
- OpenAPI remains committed in-repo, but is generated (no hand-editing).

---

## Generation Model

### Source of contract truth

For Helling-owned routes, contract truth lives in Go operation definitions:

- request and response structs
- validation tags
- operation metadata (summary, description, operationId, tags, responses)

### Generated artifact

- Output path: api/openapi.yaml
- Header must indicate generated ownership and prohibit manual edits.
- Any PR changing API behavior must include regenerated api/openapi.yaml.

### Downstream codegen

- CLI: oapi-codegen client continues unchanged.
- WebUI: hey-api/openapi-ts generates fetch client, SDK, schemas, and TanStack Query options.

---

## Why this flow

- Removes hand-authored YAML drift as a recurring class of failure.
- Reduces manual OpenAPI remediation effort from roughly 14 hours to near-zero.
- Keeps reviewability: API diffs remain visible through committed generated artifacts.

---

## Verification Gates

1. Generate OpenAPI from code.
2. Run vacuum against api/.vacuum.yaml.
3. Regenerate CLI and WebUI clients from committed api/openapi.yaml.
4. Ensure no stale generated diff remains in CI.

Reference command:

```bash
vacuum lint --ruleset api/.vacuum.yaml --fail-severity info api/openapi.yaml
```

---

## Artifact Ownership

### Generated artifacts (never hand-edit)

- api/openapi.yaml
- apps/helling-cli/internal/client/\*.gen.go
- web/src/api/generated/\*\*

### Hand-authored artifacts

- Huma operation definitions and envelopes in apps/hellingd/
- Proxy pass-through handlers for /api/incus/\* and /api/podman/\*
- Contract policy docs in docs/spec/
