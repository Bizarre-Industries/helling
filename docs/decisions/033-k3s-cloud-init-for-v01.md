# ADR-033: k3s via cloud-init for v0.1 Kubernetes provisioning

> Status: Superseded by ADR-005 (2026-04-19)

## Context

This ADR duplicated ADR-005 with equivalent scope and decision text.

## Decision

ADR-005 is the canonical decision for v0.1 Kubernetes provisioning:

- cloud-init driven k3s bootstrap on Incus VMs
- kubeconfig retrieval via Helling API/CLI
- CAPN deferred to later roadmap phases

## Consequences

- Keep a single source of truth for v0.1 Kubernetes provisioning behavior.
