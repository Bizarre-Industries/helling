# Helling Migration Manifest

Generated: 2026-04-20
Baseline commit reference: 777e4fb

Single source of truth for migration after:

- ADR-043 (Huma v2 HTTP layer)
- ADR-044 (hey-api/openapi-ts WebUI codegen)
- Tooling package landing (Taskfile, lefthook, lint configs)
- Audit drift cleanup backlog

---

## TL;DR

Helling pivots from docs-first OpenAPI authoring to code-first generation via Huma for Helling-owned routes. This removes manual remediation grind and spec drift as a recurring class of failure.

Frontend codegen pivots from orval to hey-api/openapi-ts. CLI codegen remains oapi-codegen client.

ADRs preserved: 014, 015, 040, 041.

---

## What changes

| Area               | Before                                             | After                                   |
| ------------------ | -------------------------------------------------- | --------------------------------------- |
| Backend HTTP layer | Hand-rolled handlers + hand-authored spec coupling | Huma v2 + humago on ServeMux            |
| OpenAPI            | Hand-authored YAML                                 | Generated from registered Go operations |
| Server codegen     | oapi-codegen strict server                         | Removed (Huma is the server framework)  |
| CLI codegen        | oapi-codegen client                                | Unchanged                               |
| WebUI codegen      | orval                                              | hey-api/openapi-ts                      |
| Error envelope     | Per-handler mapping                                | Centralized huma.NewError transformer   |
| OpenAPI lint       | Manual hygiene burden                              | Generated artifact regression gate      |

---

## What does not change

- ADR-014 proxy model stays.
- ADR-015 native upstream pass-through stays.
- ADR-040 ServeMux stays (Huma layered via humago).
- ADR-041 /api/v1 versioning stays.
- Auth, storage, secrets, ops, and security posture stay.

---

## ADR map

| ADR | Title                                         | Status   |
| --- | --------------------------------------------- | -------- |
| 043 | Huma with humago for Helling-owned HTTP layer | Accepted |
| 044 | hey-api/openapi-ts for WebUI code generation  | Proposed |
| 045 | APT repo tooling decision                     | Pending  |
| 046 | ISO build tooling decision                    | Pending  |
| 047 | Dark mode scope for v0.1                      | Pending  |
| 048 | Mobile scope for v0.1                         | Pending  |

---

## Phase 0 Freeze

- [x] Add pre-Huma snapshot marker to api/openapi.yaml.
- [ ] Open draft PR "chore: freeze openapi.yaml ahead of Huma pivot".
- [ ] Stop in-flight hand-remediation edits to openapi.yaml.

Acceptance:

- [x] Snapshot marker visible at top of api/openapi.yaml.

---

## Phase 1 Huma spike

Targets:

- POST /api/v1/auth/login
- GET /api/v1/users

Tasks:

- [ ] Add Huma v2 dependency.
- [ ] Bootstrap apps/hellingd with humago on ServeMux.
- [ ] Implement error envelope transformer.
- [ ] Implement generic success envelope wrapper.
- [ ] Generate OpenAPI from running app or dump tool.
- [ ] Run vacuum lint on generated artifact.

Acceptance:

- [ ] Generated OpenAPI >=95/100 (target 100/100).
- [ ] Error envelope matches docs/spec/api.md.
- [ ] Success envelope shape matches docs/spec/api.md.
- [ ] Transformer implementation remains centralized (<100 LOC target).

---

## Phase 2 Full Huma migration

Tasks:

- [ ] Migrate remaining Helling-owned operations to Huma.
- [ ] Keep /api/incus/\* and /api/podman/\* as plain handlers.
- [ ] Add deterministic OpenAPI generation target.
- [ ] Commit generated api/openapi.yaml.

Acceptance:

- [ ] vacuum 100/100 on api/openapi.yaml.
- [ ] go test ./... passes.
- [ ] Endpoint parity against planned scope is complete.

---

## Phase 3 hey-api migration

Tasks:

- [ ] Land ADR-044 acceptance decision.
- [ ] Add web/hey-api.config.ts.
- [ ] Add hey-api dependencies and gen script.
- [ ] Remove orval/axios/refine generation path.
- [ ] Regenerate web/src/api/generated.

Acceptance:

- [ ] bun run tsc --noEmit passes.
- [ ] No runtime references to orval or axios remain for generated API path.
- [ ] Frontend generated-code freshness gate passes.

---

## Phase 4 tooling package

Already drafted and expected to land:

- [ ] Taskfile.yaml
- [ ] lefthook.yml
- [ ] .golangci.yaml

Pending configs/scripts:

- [ ] .markdownlint.yaml
- [ ] .yamllint.yaml
- [ ] .shellcheckrc
- [ ] web/biome.json
- [ ] .sqlfluff
- [ ] typos.toml
- [ ] lychee.toml
- [ ] .prettierrc.yaml
- [ ] scripts/check-coverage.sh
- [ ] scripts/check-parity.sh
- [ ] scripts/install-tools.sh

Acceptance:

- [ ] task install works.
- [ ] task hooks installs hooks.
- [ ] task check passes locally.

---

## Phase 5 drift cleanup backlog

Priority Tier 0 mechanical fixes:

- [ ] Rename docs/standards/standards-quality-assurance.md to docs/standards/quality-assurance.md.
- [ ] Align docs/standards/coding.md rate-limit and pagination language with current specs.
- [ ] Align docs/standards/security.md encryption/capability/scanning sections with ADR-039 and ADR-042.
- [ ] Resolve docs/roadmap/implementation-guide.md stale architecture references (rewrite or delete).

Priority Tier 1 additions:

- [ ] docs/spec/ci.md
- [ ] docs/spec/local-dev.md
- [ ] docs/spec/pre-commit.md
- [ ] docs/standards/testing.md
- [ ] docs/standards/release.md
- [ ] docs/standards/versioning.md

Priority Tier 2 rewrites:

- [x] docs/design/openapi-pipeline.md
- [ ] docs/spec/api.md generated-artifact note
- [ ] docs/design/tools-and-frameworks.md stack updates
- [ ] CONTRIBUTING.md expansion
- [ ] docs/standards/development-environment.md expansion

---

## Verification loop

For each phase:

1. Apply changes.
2. Regenerate artifacts.
3. Run quality gates.
4. Commit only after gate pass.
5. Record unresolved drift as explicit checklist items.

---

## Standup one-liner

Pivoting backend to Huma and WebUI codegen to hey-api, while preserving ADR-014/015/040/041 and clearing outstanding standards drift via a phased checklist.
