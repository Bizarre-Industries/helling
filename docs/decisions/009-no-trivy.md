# ADR-009: Suspend Aqua Security Tooling

> Status: Accepted

## Context

March 19-23, 2026: Aqua Security supply chain attack across two distinct surfaces:

- 76 of 77 version tags in the `aquasecurity/trivy-action` GitHub Action were force-pushed to a credential stealer.
- Docker Hub images `aquasec/trivy:0.69.4`, `:0.69.5`, `:0.69.6`, and `:latest` were separately poisoned.

StepSecurity Harden-Runner detected the attack across 12,000+ repositories by monitoring outbound C2 connections. Sources: Docker blog "Trivy supply chain compromise — what Docker Hub users should know" (2026-03), Aqua advisory GHSA-69fq-xp46-6x23, StepSecurity summary.

## Decision

Suspend Trivy and other Aqua Security-hosted CI tooling in the Helling pipeline until trust and remediation criteria are explicitly re-evaluated.

Use Grype as the default container vulnerability scanner during the suspension window.

Permanent GitHub Action SHA pinning is tracked independently in ADR-026.

## Consequences

- Grype provides equivalent vulnerability scanning
- CI tooling policy separates temporary vendor suspension (this ADR) from permanent hardening policy (ADR-026)
- Must verify any new CI action is not from Aqua Security
- StepSecurity Harden-Runner recommended for detecting future supply chain attacks
