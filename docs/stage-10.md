# Stage 10 — TLS management & health

## Delivered

- TLS certs live in `/opt/browser-gateway/data/certs` (not in git); survive reboot and updates
- Install preserves `data/` across reinstall/clone
- Admin Settings: TLS module
  - Input slider: **File | Text**
  - Formats: PEM/CRT+KEY, PKCS#12; optional **chain**
  - Save & restart Traefik, or save later
  - Pending-restart banner in the web UI with restart button
- Admin Settings: **Health checks** panel (postgres / redis / docker / traefik)
- Login form fields start **empty** (no default credentials)
- Version `0.10.0`, stage `10`

## API

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/admin/tls` | Certificate status |
| PUT | `/api/admin/tls` | Upload PEM or PKCS#12 (`applyNow` optional) |
| POST | `/api/admin/tls/apply` | Restart Traefik to apply cert |
| GET | `/api/admin/health` | Aggregated health + TLS status |

## Ops notes

- Traefik container name default: `browser-gateway-traefik-1` (`TRAEFIK_CONTAINER_NAME`)
- Backend mounts `data/certs` writable; Traefik mounts the same path read-only
