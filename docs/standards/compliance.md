# Compliance Standard

This document defines compliance expectations by delivery tier, with explicit acceptance criteria and evidence artifacts.

## Tier 1 (v0.1 Ship Blockers)

These items block v0.1 release readiness.

### 1. API Contract Discipline

- Requirement: OpenAPI contract exists for the Helling API surface and is lint-clean.
- Acceptance criteria:
  - `api/openapi.yaml` includes current Helling endpoints with schemas.
  - Lint command returns zero errors.
- Evidence:
  - CI log for OpenAPI lint job.
  - Commit containing contract update and generated artifacts.

### 2. Release and Change Traceability

- Requirement: semantic versioning and conventional commits are applied.
- Acceptance criteria:
  - Tags follow `vMAJOR.MINOR.PATCH` format.
  - Commits merged to release branches follow conventional commit prefixes.
- Evidence:
  - Git tag list.
  - Generated changelog entries mapped to commit prefixes.

### 3. Licensing and Distribution Obligations

- Requirement: SPDX license clarity and AGPL obligations are documented.
- Acceptance criteria:
  - Repository has canonical license files and notices.
  - User-facing distribution docs include AGPL source-availability path.
- Evidence:
  - `LICENSE`, `NOTICE`, and release notes references.

### 4. Runtime Packaging and Filesystem Conventions

- Requirement: install/runtime paths follow Debian conventions and internal standards.
- Acceptance criteria:
  - Runtime state paths, config paths, and service units are documented and match package layout.
  - Service starts cleanly on target Debian version.
- Evidence:
  - Package manifest.
  - Installation validation output.

### 5. Security Disclosure Process

- Requirement: private disclosure workflow is present and maintained.
- Acceptance criteria:
  - `SECURITY.md` exists and defines disclosure channel and response expectations.
- Evidence:
  - `SECURITY.md` content verified in release checklist.

## Tier 2 (v0.5 Targets)

These are hardening targets for v0.5 and should be tracked as release goals.

### 1. OpenSSF and Supply-Chain Baseline

- Requirement: OpenSSF Best Practices passing level and SLSA level 1 provenance baseline.
- Acceptance criteria:
  - Badge/report indicates passing state.
  - Build provenance generated for release artifacts.
- Evidence:
  - Badge/report URL.
  - Provenance artifact attached to release.

### 2. SBOM and Vulnerability Management

- Requirement: release-attached SBOM and routine vulnerability scans.
- Acceptance criteria:
  - SBOM generated for each release artifact set.
  - Vulnerability scans run in CI with documented triage flow.
- Evidence:
  - SBOM artifact links.
  - CI scan logs and remediation issue links.

### 3. Enterprise Identity Controls

- Requirement: expanded IAM capabilities tracked and verified for target release.
- Acceptance criteria:
  - Target auth capabilities for the tier are implemented and tested.
  - Role/permission behavior is documented with test coverage.
- Evidence:
  - Spec updates.
  - Auth test results and release checklist entries.

## Tier 3 (Post-v1 Aspirations)

These are strategic quality goals and do not block v0.1/v0.5.

- SLSA level 2/3 hardened provenance
- Formal Kubernetes conformance validation flow
- WCAG AA accessibility verification program
- OpenTelemetry traces/metrics correlation
- CloudEvents event contract standardization
- Full OWASP API Top 10 verification program

## Priority Rule

When a requirement appears in multiple places, this tier map decides release blocking priority.

- Tier 1: release-blocking for v0.1
- Tier 2: release-goal for v0.5
- Tier 3: strategic backlog beyond v1
