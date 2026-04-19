# ADR-010: noVNC as primary VM browser console, SPICE as fallback

> Status: Accepted (2026-04-19)

## Context

Helling needs a browser-native VM console that works reliably in common desktop browsers without client installation.

SPICE in browser depends on aging client implementations and inconsistent compatibility. noVNC is broadly deployed, active, and aligned with expected browser console UX.

Incus default VM console wiring may use SPICE, but Helling controls VM launch parameters and can expose a VNC socket for browser use.

## Decision

Use noVNC as the default in-browser VM console in v0.1.

Implementation:

1. VM definitions include QEMU VNC socket configuration (`raw.qemu`) on a local Unix socket.
2. hellingd proxies that socket to WebSocket for browser clients.
3. WebUI uses noVNC as the primary viewer.
4. SPICE remains available only as an optional external-client fallback (for example, `.vv` download workflow).

## Consequences

- Browser console path is standardized around noVNC
- Console experience matches operator expectations from comparable virtualization UIs
- Helling must maintain secure socket-to-WebSocket proxying and access checks
- SPICE remains optional, not the default dependency for WebUI console
