# Security Baseline (Stage 1)

## Trust boundaries

| Zone | Trust |
|------|-------|
| User browser | Untrusted |
| Traefik + frontend | Semi-trusted (public) |
| Backend API | Trusted |
| PostgreSQL / Redis | Trusted, internal network only |
| Browser containers | **Untrusted workloads** (user-controlled web content) |
| Docker socket | Mounted **only** into backend/orchestrator |

Browser containers must never receive: Docker socket, host PID/net, privileged flag, or writable host paths.

## Container isolation (required)

```yaml
privileged: false
network_mode: bridge   # never host
cap_drop:
  - ALL
security_opt:
  - no-new-privileges:true
read_only: true
# tmpfs for required writable paths
user: non-root
```

No `cap_add` unless a Stage 6/7 spike proves a minimal capability is mandatory (document exception).

## AuthN / AuthZ

- Passwords: argon2id
- Access JWT: short-lived (e.g. 15m)
- Refresh JWT: rotating, stored hashed server-side, revoke on logout
- Roles v1: `SUPER_ADMIN`, `USER`
- USER may only control own sessions
- SUPER_ADMIN: admin APIs + any session stop
- Per-session **agent token** for container → backend internal calls

## API protections (implementation stages)

- CSRF: same-site cookies if cookie mode used; prefer Bearer tokens from memory for SPA
- Rate limit: login + create session (Redis)
- Brute-force: lockout / backoff on login
- Session timeout: idle + max duration settings
- Password policy: min length + complexity (settings)

## HTTP security headers (Traefik/middleware)

Public edge is **HTTPS only** (0.17.2+): Traefik `:80` permanently redirects to `:443`.

- `Strict-Transport-Security` (HSTS; `forceSTSHeader`)
- `Content-Security-Policy` (tight for app; session viewer may need media exceptions; `connect-src` allows `wss:`)
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `X-Frame-Options: SAMEORIGIN` (allows first-party noVNC iframe; blocks third-party embedding)
- Permissions-Policy limiting camera/mic as needed

## Audit & privacy

- Audit toggles in settings (incl. keystrokes **default off**)
- Retention budget default 10 GiB with rotation (oldest first)
- History retention days (default 30; `0` = until manual admin delete)
- No video recording in v1
- Network panel + history data may contain sensitive URLs — access limited to session owner + SUPER_ADMIN

## Secrets

- DB/Redis passwords via env / Compose secrets
- JWT signing key required at boot
- Self-signed TLS for v1; replace with real CA in hardening / ACME (backlog)
- Download ZIP default password stored like TI API keys (set flag, never echo raw)
- Agent tokens single-use per session, rotated on recreate

## Explicit non-goals for v1 security

- IP allowlists
- MFA
- External IdP
- Multi-tenant hard isolation
- Domain egress allowlist (deferred feature)
