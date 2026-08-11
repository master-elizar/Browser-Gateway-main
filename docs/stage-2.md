# Stage 2 — Backend Skeleton

## Delivered

- Go Fiber backend entrypoint (`backend/cmd/server`)
- Config via env (`JWT_SECRET` required)
- PostgreSQL + Redis store with AutoMigrate for core tables
- Health endpoints: `GET /healthz`, `GET /readyz`, `GET /api/version`
- API route stubs returning `501` for auth/browser/admin (later stages)
- Orchestrator docker socket ping (lifecycle in Stage 5)
- Docker Compose stack: Traefik, frontend stub, backend, postgres, redis
- Self-signed TLS certs for `browser-gateway.local` / localhost
- Install script: `scripts/install.sh` (GitHub one-liner → `/opt/browser-gateway`)

## Deploy target (dev)

| Item | Value |
|------|-------|
| Host | set via `BG_HOST` |
| User | set via `BG_USER` |
| Path | `/opt/browser-gateway` (override with `BG_REMOTE_DIR`) |
| URL | `https://<host>/` (self-signed; HTTP redirects to HTTPS) |

### Notes

- Traefik uses **file provider** (not Docker provider): Docker Engine 29 rejects Traefik’s embedded Docker API client.
- Backend joins docker group (GID must match the host Docker socket group) for socket access.
- Size the host for Chromium sessions (RAM/CPU) before load testing.

## Local commands

```bash
# generate certs
./scripts/gen-certs.sh

# bare-host install (Ubuntu/Debian)
sudo ./scripts/install.sh
```

## Verify

```bash
curl -sk https://<host>/healthz
curl -sk https://<host>/readyz
curl -sk https://<host>/api/version
```
