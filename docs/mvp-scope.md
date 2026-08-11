# MVP Scope (v1) — Locked

> Source: discovery answers, 2026-08-06. Changes require explicit re-approval.

## Product

| Item | Decision |
|------|----------|
| Product type | Personal SaaS (single-tenant) |
| Concurrent users | ≥ 5 |
| Tabs per browser | up to ~40 |
| Sessions per user | Multiple parallel |
| Collaboration / invite | Deferred |
| Mobile | Out of scope |
| Branding / white-label | None |

## Browser model

| Item | Decision |
|------|----------|
| Engine | Real Chromium in Docker (no HTML emulation, no iframe, no screenshot streaming) |
| Profile | Temporary only — destroy container + data on session end |
| Extensions | None (clean browser) |
| Domain allowlist | Deferred |
| Isolation | Full: separate container, network ns, filesystem, non-root user |

## Streaming & UX

| Item | Decision |
|------|----------|
| Primary stream | WebRTC (video watching must work) |
| Fallback | WebSocket + noVNC |
| Chrome-like overlay UI | No — clean remote desktop of Chromium |
| Session toolbar | Clipboard sync, Upload, Download manager, Stop, Network monitor |
| Languages | EN + RU |

## Network observability (any.run-lite)

| Item | Decision |
|------|----------|
| DNS log (realtime) | In scope |
| HTTP(S) method/URL/status/headers | In scope |
| UI | Side panel on session view |
| PCAP download | Deferred |
| Video session recording | Deferred |

## Auth & admin

| Item | Decision |
|------|----------|
| Auth | Local users + JWT (access + refresh) |
| MFA / LDAP / OIDC | Deferred |
| Roles (v1) | `SUPER_ADMIN`, `USER` |
| Admin panel | Minimal: Users, Sessions, Settings, Audit |
| Settings | Log toggles, retention (~10 GB default), app parameters |

## Audit

| Item | Decision |
|------|----------|
| Events | Session open/close, control actions, URLs, downloads, network (per toggles) |
| Retention | Configurable in web UI; default budget ~10 GB then rotate |
| Video recording | Deferred |

## Infrastructure (v1)

| Item | Decision |
|------|----------|
| Orchestration | Docker Compose, single host |
| Workers | Same host (multi-node later) |
| TLS | Self-signed |
| Monitoring stack (Prometheus/Grafana/Loki) | Stage 9 — not blocking MVP core |
| GPU encode | Not required; software encoder first |

## Explicitly out of v1

- Collaborative sessions / invite links
- Domain allowlist / egress policy UI
- Persistent browser profiles
- MFA, LDAP, AD, OIDC/Keycloak
- Multi-tenant isolation
- Multi-node browser workers
- Full resource admin (per-container CPU/RAM dashboards as originally sketched)
- PCAP, video recording
- Mobile clients

## Capacity target (design assumption)

- 5 users × several sessions ≈ **up to ~15 concurrent browser containers** on one host for v1 sizing
- Each container: Chromium + Xvfb + WebRTC agent + network tap
- Host sizing recommendation documented in `docs/architecture.md`
