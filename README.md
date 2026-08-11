# Browser Gateway

**Chrome Remote Browser as a Service** — real Chromium in Docker, streamed to the user over WebRTC (noVNC fallback).

## Status

**Current release: 0.17.x** (Stage 16). Catch-up through Stage 16 is complete. Next ideas: [docs/backlog.md](docs/backlog.md). Full doc index: [docs/README.md](docs/README.md).

| Stage | Description | Status |
|-------|-------------|--------|
| 1 | Architecture, API, scope | Done |
| 2 | Backend skeleton + Compose | Done |
| 3 | Frontend | Done |
| 4 | Authentication | Done |
| 5 | Browser container lifecycle | Done |
| 6 | Chromium engine | Done |
| 7 | Stream + toolbar + netmon | Done (+ WebRTC) |
| 8 | Minimal admin | Done |
| 9 | Monitoring (Prometheus + Grafana + Loki) | Done |
| 10 | TLS management + health panel | Done |
| 11 | First-run setup + in-app updates | Done |
| 12 | UI polish + live activity logs | Done |
| 13 | Threat intelligence (VirusTotal) | Done |
| 14 | Multi-provider TI | Done |
| 15 | Network request tree | Done |
| 16 | Session history, download formats, paste, HTTPS-only | Done (0.17.x) |

UI is served only over **HTTPS** (`:443`). Port `:80` permanently redirects to HTTPS. Monitoring: Grafana `:3000` — see [docs/stage-9.md](docs/stage-9.md).

## Install (bare Ubuntu/Debian)

Everything lands under `/opt/browser-gateway`.

```bash
curl -fsSL https://raw.githubusercontent.com/TaskMaster329/Browser-Gateway/main/scripts/install.sh | sudo bash
```

Optional: set the address clients use for TURN/Grafana:

```bash
curl -fsSL https://raw.githubusercontent.com/TaskMaster329/Browser-Gateway/main/scripts/install.sh | sudo BG_PUBLIC_HOST='your.host.or.ip' bash
```

After install:

| Item | Path / command |
|------|----------------|
| App + configs | `/opt/browser-gateway` |
| Admin login | first-run `/setup` with one-time key from installer |
| Certs | `/opt/browser-gateway/data/certs` |
| History frames | `/opt/browser-gateway/data/history/` |
| Compose env | `/opt/browser-gateway/deploy/.env` |
| Backend env | `/opt/browser-gateway/configs/backend.env` |
| Service | `systemctl start/stop/restart/reload/status browser-gateway` |
| UI | `https://<host>/` (self-signed) |
| Updates | Admin → Settings → Updates (pulls `main`, rebuilds) |

Build a local `.deb` (Linux or Docker):

```bash
./scripts/build-deb.sh
sudo apt install ./dist/browser-gateway_0.9.5_amd64.deb
```

## Documentation

- [Docs index](docs/README.md)
- [Overview (RU)](docs/overview.ru.md)
- [Architecture](docs/architecture.md)
- [API](docs/api.md)
- [Security](docs/security.md)
- [Session lifecycle](docs/session-lifecycle.md)
- [Stage 16 — History / downloads / paste / HTTPS](docs/stage-16.md)
- [Backlog](docs/backlog.md)

## Stack

| Layer | Tech |
|-------|------|
| Backend | Go 1.23+, Fiber, REST + WebSocket |
| Frontend | React, TypeScript, Vite, TailwindCSS |
| DB / Cache | PostgreSQL, Redis |
| Edge | Traefik (HTTPS + HSTS; HTTP→HTTPS redirect) |
| Engine | Chromium + Xvfb + WebRTC agent + CDP + dnsmasq + netmon |
| Deploy | Docker Compose (single host) |

## Repository layout

```
backend/          Go API + orchestrator
frontend/         Web UI
browser-engine/   Chromium / Firefox container images
deploy/           Compose + Traefik + monitoring
configs/          Example configuration
docs/             Architecture & stage notes
scripts/          Install, update, certs, deb
tests/            Integration / e2e
```

## Product in one sentence

Local-auth users launch isolated temporary Chromium sessions, watch them over WebRTC (noVNC fallback), control clipboard/upload/download, inspect DNS + HTTP(S) + TI, and review closed sessions in History (screenshots filmstrip + the same Network panel); super-admin manages users, sessions, settings, retention, and audit.
