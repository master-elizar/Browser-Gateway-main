# API Draft — Browser Gateway (Stage 1)

Base URL: `https://<host>/api`  
WebSocket base: `wss://<host>/ws`  
Auth: `Authorization: Bearer <access_token>` unless noted.

Public edge is HTTPS-only (HTTP redirects to HTTPS).

All JSON request/response bodies use `camelCase` field names in HTTP JSON.

Errors:

```json
{
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "Session not found"
  }
}
```

---

## 1. Auth

### `POST /auth/register`

Bootstrap / super-admin gated later. v1: first user becomes `SUPER_ADMIN` if no users exist; further registration disabled unless `settings.allowRegistration=true`.

```json
{ "email": "admin@example.com", "password": "…", "displayName": "Admin" }
```

### `POST /auth/login`

```json
{ "email": "…", "password": "…" }
```

Response:

```json
{
  "accessToken": "…",
  "refreshToken": "…",
  "user": { "id": "…", "email": "…", "role": "SUPER_ADMIN", "displayName": "…" }
}
```

### `POST /auth/refresh`

```json
{ "refreshToken": "…" }
```

### `POST /auth/logout`

Invalidates refresh token.

### `GET /auth/me`

Current user profile.

---

## 2. Browser sessions

### `POST /browser/create`

Creates session row and starts container (async). Optional body:

```json
{
  "name": "Research",
  "startUrl": "https://example.com"
}
```

Response `201`:

```json
{
  "id": "847392",
  "status": "CREATING",
  "name": "Research",
  "createdAt": "…"
}
```

### `POST /browser/{id}/start`

Idempotent start if stopped mid-flight (v1 mostly used after create). Transitions toward `RUNNING`.

### `POST /browser/{id}/stop`

Graceful stop → `STOPPING` → `DESTROYED` (v1 does not keep stopped containers warm).

### `DELETE /browser/{id}`

Force destroy if not already `DESTROYED`.

### `GET /browser/list`

Query: `status`, `mine=true` (default for USER), pagination.

```json
{
  "items": [
    {
      "id": "847392",
      "name": "Research",
      "status": "RUNNING",
      "ownerId": "…",
      "containerId": "…",
      "startedAt": "…",
      "durationSec": 120
    }
  ],
  "total": 1
}
```

### `GET /browser/{id}`

Detail + signaling endpoints.

```json
{
  "id": "847392",
  "status": "RUNNING",
  "signalingUrl": "/ws/sessions/847392/signaling",
  "netmonUrl": "/ws/sessions/847392/netmon",
  "controlUrl": "/ws/sessions/847392/control"
}
```

---

## 3. Session controls (HTTP)

Used by toolbar; some also available over WS control channel.

### `POST /browser/{id}/clipboard`

```json
{ "direction": "toRemote" | "fromRemote", "text": "…" }
```

`fromRemote` returns `{ "text": "…" }`.

### `POST /browser/{id}/upload`

`multipart/form-data` file → placed into remote download/upload drop directory and optionally opened.

### `GET /browser/{id}/downloads`

List files available to pull from the session.

### `GET /browser/{id}/downloads/{fileId}`

Download file to client.

### `GET /browser/{id}/network/events`

Historical netmon events (pagination, filters: `type=dns|http`).

---

## 4. WebSocket protocols

### `/ws/sessions/{id}/signaling`

WebRTC SDP/ICE exchange.

Client → server examples:

```json
{ "type": "offer", "sdp": "…" }
{ "type": "ice", "candidate": { } }
```

Server → client:

```json
{ "type": "answer", "sdp": "…" }
{ "type": "ice", "candidate": { } }
{ "type": "status", "state": "RUNNING" }
```

### `/ws/sessions/{id}/control`

Optional channel for input if not using RTC datachannel; also clipboard notifications.

```json
{ "type": "pointer", "x": 10, "y": 20, "buttons": 1 }
{ "type": "key", "down": true, "key": "a", "code": "KeyA" }
```

### `/ws/sessions/{id}/netmon`

Server pushes:

```json
{
  "type": "dns",
  "ts": "…",
  "query": "example.com",
  "qtype": "A",
  "answers": ["<resolved-address>"]
}
```

```json
{
  "type": "http",
  "ts": "…",
  "method": "GET",
  "url": "https://example.com/",
  "status": 200,
  "requestHeaders": { },
  "responseHeaders": { }
}
```

---

## 5. Admin (SUPER_ADMIN)

### Users

- `GET /admin/users`
- `POST /admin/users` — create user `{ email, password, role, displayName }`
- `PATCH /admin/users/{id}` — role, active flag, reset password
- `DELETE /admin/users/{id}`

### Sessions

- `GET /admin/sessions` — all users’ sessions
- `POST /admin/sessions/{id}/stop`

### Settings

- `GET /admin/settings`
- `PUT /admin/settings`

Example settings payload:

```json
{
  "maxConcurrentSessionsGlobal": 15,
  "maxConcurrentSessionsPerUser": 3,
  "idleTimeoutSec": 1800,
  "maxSessionDurationSec": 14400,
  "audit": {
    "retentionBytes": 10737418240,
    "logSessionLifecycle": true,
    "logControlActions": true,
    "logVisitedUrls": true,
    "logDownloads": true,
    "logNetworkDns": true,
    "logNetworkHttp": true,
    "logKeystrokes": false
  },
  "allowRegistration": false
}
```

### Audit

- `GET /admin/audit` — filters: `userId`, `sessionId`, `from`, `to`, `type`
- `GET /admin/audit/export` — optional CSV/JSON export

---

## 6. Internal (container → backend)

Authenticated with per-session agent token (not user JWT).

- `POST /internal/sessions/{id}/health`
- `POST /internal/sessions/{id}/netmon` — batch events
- `POST /internal/sessions/{id}/downloads/notify`
- `POST /internal/sessions/{id}/history/frame` — screenshot capture (Stage 16)

Never exposed publicly via Traefik; only on Docker network.

---

## 7. Session history (0.17+)

Authenticated with user JWT. ACL: owner sees own closed sessions; `SUPER_ADMIN` sees all. Delete is SUPER_ADMIN only.

### `GET /history`

Query: `from`, `to`, `name`, `browser`, `tiVerdict`.

Lists `DESTROYED` sessions with frame counts / worst TI verdict summary.

### `GET /history/{id}`

Session meta + `frames` + `network` + `audit` for timeline / filmstrip / Network panel.

### `GET /history/{id}/frames/{eventId}`

Serves the JPEG for a timeline frame (Bearer auth).

### `DELETE /history/{id}`

SUPER_ADMIN: removes timeline rows, network/audit for that session, and `data/history/{id}/`.

### Internal

- `POST /internal/sessions/{id}/history/frame` — agent uploads JPEG + meta (`kind`: `click` | `navigate`)

---

## 8. Health

- `GET /healthz` — liveness
- `GET /readyz` — postgres + redis + docker reachability
