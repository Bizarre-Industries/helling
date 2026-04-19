# ADR-006: Defer microVM support from v0.1

> Status: Accepted (2026-04-19)

## Context

Cloud Hypervisor microVM integration is attractive for fast-boot ephemeral workloads, but it adds non-trivial lifecycle management, networking behavior differences, image format constraints, and additional operator complexity.

Helling v0.1 is focused on stable Incus VM/LXC and Podman workflows.

## Decision

Do not ship first-class microVM support in v0.1.

- No Cloud Hypervisor workload type in v0.1 UI, API, or CLI
- No per-microVM socket routing layer in v0.1 proxy
- No microVM-specific state tables in v0.1 schema

MicroVM support can be revisited in v0.5+ after core platform features and lifecycle operations are stable.

## Consequences

- Reduced v0.1 implementation and testing scope
- Avoids introducing a fourth compute path before baseline maturity
- Keeps compute model aligned with current docs-first priorities
- Future microVM work requires a new ADR and explicit API/CLI/WebUI parity plan
