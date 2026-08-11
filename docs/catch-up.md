# Catch-up — Stages 1–9 fill-ins (historical)

> Status: **complete**. Kept for history of the catch-up program after early MVP cuts.

## Phase 1 — Policies

- Redis rate limit (login + session create) + login lockout
- Idle / max-duration policy worker
- Retention rotation worker
- Audit toggles enforced (sessions + netmon)
- Password min length / complexity settings
- CSP + Permissions-Policy (Traefik)
- `GET /admin/audit/export`

## Phase 2 — DNS

- dnsmasq in browser-engine + `dns_tap.py`
- Container DNS forced to `127.0.0.1`

## Phase 3 — WebRTC

- coturn default-on
- Signaling WS + ICE API
- `webrtc_agent.py` (aiortc) + frontend `WebRTCViewer`
- noVNC remains as fallback toggle

## Phase 4 — Control

- Control WS → `control_agent.py` (xdotool)
- Keystroke audit when toggle enabled

## Phase 5 — Isolation / ops

- ReadonlyRootfs + tmpfs; CPU/RAM from env; `NET_BIND_SERVICE` for dnsmasq
- Orphan reconcile worker
- Removed `AdminStub`; README updated

## Phase 6 — Observability

- Loki + Promtail + Alertmanager + Grafana Loki datasource

Later stages (10–16) are documented under `docs/stage-*.md`.
