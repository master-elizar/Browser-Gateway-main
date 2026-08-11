# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Browser Gateway is "Chrome Remote Browser as a Service" — per-session Chromium containers streamed to users via WebRTC/noVNC, fronted by Traefik. It is a multi-service application, not a monorepo; each top-level directory is a separate build unit with its own Dockerfile:

- `backend/` — Go 1.23 (module `github.com/browser-gateway/backend`), Fiber-based REST + WebSocket API, session orchestrator, auth, threat-intel integrations (`internal/ti/`: abuseipdb, otx, spamhaus, threatfox, urlhaus, virustotal).
- `frontend/` — React 19 + TypeScript + Vite 6 + Tailwind 4 SPA.
- `browser-engine/` — Python agents (`control_agent.py`, `dns_tap.py`, `health_agent.py`, `session_agent.py`, `webrtc_agent.py`) running inside the per-session Chromium container. No dependency manifest in this directory — deps are installed inline in its Dockerfile.
- `deploy/` — `docker-compose.yml` orchestrating all services (traefik, frontend, backend, postgres, redis, coturn, browser-engine, browser-engine-firefox, prometheus, grafana, loki, promtail, alertmanager), plus Traefik dynamic config and the monitoring stack.
- `packaging/` — Debian packaging (`debian/control`, `postinst`, `prerm`) and systemd units.
- `scripts/` — `install.sh`, `build-deb.sh`, `apply-update.sh`, `gen-certs.sh`, `systemd-exec.sh`.
- `docs/` — architecture, api, security, session-lifecycle, mvp-scope, and per-stage implementation notes. Start at `docs/README.md` for the index.

Production install is via `scripts/install.sh` on bare Ubuntu/Debian (installs to `/opt/browser-gateway`), not a plain `git clone && docker compose up` flow, though Compose is the underlying mechanism.

## Build commands

- Backend: `go build -o /out/browser-gateway ./cmd/server` (run from `backend/`; matches the Dockerfile build step, preceded by `go mod tidy`).
- Frontend: `npm run build` (runs `tsc --noEmit && vite build`). `npm run dev` for local dev server. There is no `lint` or `test` script defined in `frontend/package.json`.
- Debian package: `./scripts/build-deb.sh`.

## Current gaps to be aware of

- **No CI pipeline** — nothing in-repo automates or verifies build/test steps.
- **No lint/format tooling configured** for any language (no ESLint/Prettier/Biome config, no golangci-lint, no ruff/flake8). Don't assume a formatter will catch style issues.
- **Tests are not part of the regular workflow.** `backend/` has some `_test.go` files but they aren't routinely run; `tests/` at the repo root is an empty placeholder despite the README describing it as "Integration / e2e". Don't assume test coverage exists or that changes are verified by a suite.
- No `CONTRIBUTING.md` and no established branch/commit/PR conventions.

## Security-sensitive constraints

Per `docs/security.md`, browser containers must remain strictly isolated: `privileged: false`, `cap_drop: ALL`, `read_only: true`, run as non-root, and never get the Docker socket mounted into them — the socket is only ever mounted into the backend/orchestrator. Treat any change touching `deploy/docker-compose.yml` or container definitions with this in mind.

The UI is HTTPS-only; Traefik permanently redirects `:80` → `:443`.

Real secrets/certs must stay untracked (see `.gitignore`): `.env`, `*.pem`, `*.key`, `configs/backend.env`, `deploy/.env`, `data/certs/*`. Use `configs/backend.env.example` and `deploy/.env.example` as references for required env vars, never commit real values.
