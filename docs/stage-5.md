# Stage 5 — Browser container lifecycle

## Delivered

- Docker orchestrator: create / start / health-wait / stop / destroy
- Session API: `create`, `list`, `get`, `stop`, `delete`
- Admission limits (global + per-user) from `app_settings`
- Isolated stub containers (`browser-gateway/browser-engine:local`) with:
  - `cap_drop: ALL`
  - `read_only`
  - `no-new-privileges`
  - tmpfs `/tmp`
  - non-root user
- Health agent on `:8090/healthz` → session becomes `RUNNING`
- Frontend polls session status; Launch/Stop wired

## Not yet (Stage 6/7)

- Real Chromium
- WebRTC video/input
- Netmon DNS/HTTP panel data

## Verify

1. Login at `https://<host>/`
2. **Launch Browser**
3. Status should move `CREATING → STARTING → RUNNING`
4. `docker ps` shows `browser-session-*`
5. **Stop** removes container and marks `DESTROYED`
