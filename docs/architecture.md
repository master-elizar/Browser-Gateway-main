# Architecture — Browser Gateway (Stage 1)

## 1. Goal

Browser Gateway is a **Chrome Remote Browser as a Service** platform.

The user authenticates at the gateway web UI, clicks **Launch Browser**, and receives a **live WebRTC stream** of a real Chromium instance running in an isolated Docker container — not an HTML fake browser, not an iframe, not screenshot polling.

Closest product references: Kasm Workspaces, BrowserBox, Apache Guacamole, Chrome Remote Desktop; network side-panel inspired by any.run (DNS + HTTP(S) timeline).

## 2. High-level diagram

```
                         User Browser (desktop)
                                  |
                               HTTPS
                          (Traefik TLS)
                                  |
                    +-------------v--------------+
                    |      Browser Gateway       |
                    |  frontend (React/Vite)     |
                    |  backend  (Go / Fiber)     |
                    +------+------+------+-------+
                           |      |      |
              +------------+      |      +------------+
              |                   |                   |
              v                   v                   v
        PostgreSQL              Redis           Docker Engine
        (users, sessions,    (session state,    (browser containers)
         audit, settings)     pub/sub, locks)
                                  |
                                  |
                    +-------------v--------------+
                    |   Browser Orchestrator     |
                    |   (in backend process)     |
                    +-------------+--------------+
                                  |
              +-------------------+-------------------+
              |                   |                   |
              v                   v                   v
     browser-session-A   browser-session-B   browser-session-C
     +---------------+   +---------------+   +---------------+
     | Chromium      |   | Chromium      |   | Chromium      |
     | Xvfb          |   | Xvfb          |   | Xvfb          |
     | WebRTC agent  |   | WebRTC agent  |   | WebRTC agent  |
     | CDP bridge    |   | CDP bridge    |   | CDP bridge    |
     | netmon agent  |   | netmon agent  |   | netmon agent  |
     +-------+-------+   +-------+-------+   +-------+-------+
             |                   |                   |
             +---------+---------+---------+---------+
                       |                   |
                       v                   v
                 WebRTC media         netmon events
                 (to user)            (WS → UI panel)
```

## 3. Components

### 3.1 `frontend/`

- React + TypeScript + Vite + TailwindCSS
- i18n: EN / RU
- Surfaces:
  - Login / first-run setup
  - Session list + Launch Browser
  - Session viewer (WebRTC / noVNC + toolbar + network panel)
  - Session history (filmstrip + read-only network)
  - Admin: Users, Sessions, Settings, Audit, Updates, TLS

### 3.2 `backend/` (Go 1.23+, Fiber)

Single deployable API process for v1 (split later if needed).

Modules:

| Module | Responsibility |
|--------|----------------|
| `auth` | Local users, password hashing (argon2id), JWT access/refresh |
| `users` | CRUD (super-admin), role assignment |
| `sessions` | Browser session lifecycle FSM |
| `orchestrator` | Create/start/stop/destroy Docker browser containers |
| `signaling` | WebRTC signaling over WebSocket |
| `netmon` | Ingest DNS/HTTP events from container agents |
| `audit` | Append-only audit events + retention worker |
| `settings` | App settings (retention, log toggles) |
| `ws` | Multiplexed WS hub (signaling, netmon, control ACKs) |

`session-manager` from the original sketch is **not a separate process in v1** — it lives as the `sessions` + `orchestrator` modules inside `backend/`. Multi-process split is a later scaling step.

### 3.3 `browser-engine/`

Docker image baked once, instantiated per session:

| Process | Role |
|---------|------|
| Xvfb | Virtual display |
| Chromium | Real browser (CDP enabled on localhost inside container) |
| webrtc-agent | Captures display / injects input; WebRTC to client |
| cdp-bridge | Optional thin helper for tab/download hooks via CDP |
| netmon-agent | Captures DNS + HTTP(S) metadata; streams to gateway |

Image constraints (hard requirements):

```yaml
privileged: false
network: bridge (custom per-session or shared internal; never host)
cap_drop: [ALL]
# add only minimal caps if encoder/input absolutely requires (documented)
security_opt: ["no-new-privileges:true"]
read_only: true
tmpfs: ["/tmp", "/home/browser"]
# no docker.sock mount
# no host filesystem bind except optional empty named volume wiped on destroy
```

### 3.4 Data stores

**PostgreSQL**

- `users`
- `refresh_tokens`
- `browser_sessions`
- `audit_events`
- `app_settings`
- `session_network_events` (optional short-TTL table; or Redis stream + sample to PG)

**Redis**

- Session runtime state (`RUNNING` metadata, container id, worker lease)
- Pub/Sub: netmon fan-out, session control events
- Distributed locks for start/stop (prep for multi-node)
- Rate-limit counters

### 3.5 Edge: Traefik

- HTTPS with self-signed cert (v1)
- Routes:
  - `/` → frontend
  - `/api/*` → backend
  - `/ws/*` → backend (WebSocket sticky)
- Security headers (CSP, HSTS optional for self-signed, X-Frame-Options, etc.)

## 4. Request / media flows

### 4.1 Launch browser

```
User → POST /api/browser/create
     → backend creates DB row (CREATING)
     → orchestrator docker create/start (browser-engine image)
     → wait healthy (CDP + webrtc-agent ready)
     → status RUNNING
     → returns session_id + ws signaling URL
User → open Session Viewer
     → WS /ws/sessions/{id}/signaling
     → WebRTC offer/answer + ICE
     → media flows peer-to-peer or via gateway TURN (see §7)
```

### 4.2 Input (mouse / keyboard)

```
User events → WebRTC datachannel (preferred) or WS control
           → webrtc-agent → Xvfb/Chromium
```

Clipboard / upload / download use explicit toolbar actions (HTTP or datachannel), not silent OS sync.

### 4.3 Network monitor

```
Container netmon-agent
  → DNS (via local resolver hook or eBPF/nft log — implementation in Stage 6/7)
  → HTTP(S) metadata (mitm optional OR Chromium Network CDP domain)
  → WS/HTTP push to backend /internal/netmon
  → Redis pub/sub
  → frontend side panel
```

**v1 preferred approach for HTTP(S):** Chromium **CDP Network** domain (accurate request list without breaking TLS). DNS via container resolver logging (`dnsmasq`/`coredns` sidecar or Chromium DNS hooks). Full transparent MITM and PCAP deferred.

### 4.4 Stop / destroy

```
POST stop or DELETE
  → STOPPING
  → docker stop + rm
  → wipe tmp volumes
  → audit event
  → DESTROYED
```

## 5. Session lifecycle (FSM)

```
CREATING → STARTING → RUNNING ⇄ IDLE → STOPPING → DESTROYED
                \         |
                 \--------/ (failure → STOPPING → DESTROYED)
```

| State | Meaning |
|-------|---------|
| `CREATING` | DB row + container create |
| `STARTING` | Processes booting; healthchecks |
| `RUNNING` | WebRTC available; user connected or connectable |
| `IDLE` | Running but no connected viewer beyond idle timeout |
| `STOPPING` | Teardown in progress |
| `DESTROYED` | Terminal; resources gone |

Idle timeout and max session duration are `app_settings`.

Detail: `docs/session-lifecycle.md`.

## 6. Docker Compose (v1 topology)

Services:

| Service | Image / build | Notes |
|---------|---------------|-------|
| `traefik` | traefik | TLS, routing |
| `frontend` | build `frontend/` | static nginx or vite preview → nginx |
| `backend` | build `backend/` | Fiber API + orchestrator |
| `postgres` | postgres:16 | persistent volume |
| `redis` | redis:7 | persistent optional |
| `browser-*` | build `browser-engine/` | **not** static compose services — created dynamically by orchestrator |

Dynamic containers join an internal Docker network `browser-net` that backend can reach for CDP/signaling agent ports; **not** published to the host by default.

```
deploy/docker-compose.yml
configs/traefik/
configs/backend.env.example
docker/browser-engine/Dockerfile
```

## 7. WebRTC topology (v1)

On a single host / LAN-friendly SaaS:

1. **Preferred:** client ↔ browser-agent **direct** WebRTC (host ICE candidates via published UDP range **or** agent connected through gateway TURN).
2. **Pragmatic v1:** backend embeds / runs **coturn** (or pion TURN) so media always relays through the gateway host — simpler NAT story for “open URL and it works”.

Decision for implementation stages: **include coturn in Compose** for reliability; optimize to direct ICE later.

## 8. Scaling path (post-v1, designed-in)

```
Gateway (API) → Scheduler → Worker Node 1..N → browser containers
```

v1 uses local Docker SDK (`/var/run/docker.sock` **only on backend service**, never inside browser containers). Socket access is a privileged trust boundary limited to the orchestrator container.

Interfaces to keep stable for later multi-node:

- `Orchestrator` interface: `Create / Start / Stop / Destroy / Health`
- Redis locks + session→worker affinity
- Agent callback auth (per-session token)

## 9. Project layout

```
Browser-Gateway/
├── README.md
├── docs/
│   ├── mvp-scope.md
│   ├── architecture.md      ← this file
│   ├── api.md
│   ├── session-lifecycle.md
│   └── security.md
├── backend/                 # Go Fiber API + orchestrator
├── frontend/                # React TS Vite
├── browser-engine/          # Chromium container image sources
├── docker/                  # shared Docker helpers
├── deploy/                  # compose, traefik, prod overlays
├── configs/                 # example env, policies
├── scripts/                 # dev bootstrap, cert gen
└── tests/                   # e2e / integration
```

## 10. Tech choices (locked for Stage 2+)

| Area | Choice | Rationale |
|------|--------|-----------|
| API | Go 1.23 + Fiber | Fast WS + REST, simple deploy |
| Frontend | React + TS + Vite + Tailwind | Matches brief |
| DB | PostgreSQL 16 | Relational audit/users |
| Cache | Redis 7 | Pub/sub + locks |
| Browser | Chromium + CDP | Real browser control |
| Automation helper | Playwright optional inside engine | Prefer CDP directly for lower overhead; Playwright only if it speeds engine bring-up |
| Stream | WebRTC (pion on agent or proven gstreamer/WebRTC stack) | Low latency + video |
| Fallback | websockify + noVNC | Compatibility |
| Proxy | Traefik | WS + TLS |
| Passwords | argon2id | Modern default |
| Tokens | JWT access (short) + refresh (rotating) | Spec |

## 11. Host sizing (guidance)

For ~15 concurrent sessions (5 users × ~3 sessions):

| Resource | Rough guide |
|----------|-------------|
| CPU | 16+ cores recommended (Chromium is heavy) |
| RAM | 64 GB recommended (~2–4 GB per session headroom) |
| Disk | SSD; images + 10 GB log budget + container writable layers |
| Network | UDP open for TURN/WebRTC |

Exact limits will be enforced via orchestrator admission control (`max_concurrent_sessions` setting).

## 12. Stage map

| Stage | Deliverable |
|-------|-------------|
| 1 | Architecture + API + scope (this) |
| 2 | Backend skeleton + Compose wiring |
| 3 | Frontend shell + i18n |
| 4 | Local auth + JWT + roles |
| 5 | Container lifecycle orchestrator |
| 6 | Chromium engine image + CDP |
| 7 | WebRTC + toolbar + netmon panel |
| 8 | Minimal admin (users/sessions/settings/audit) |
| 9 | Metrics / logs (Prometheus + Grafana + Loki) |
| 10 | TLS management + health |
| 11 | First-run setup + in-app updates |
| 12 | UI polish + live activity |
| 13–14 | Threat intelligence |
| 15 | Network request tree |
| 16 | Session history, download formats, paste, HTTPS-only |

See [README.md](README.md) for the docs index and [stage-16.md](stage-16.md) for the current line.

## 13. Acceptance criteria for Stage 1

- [x] MVP scope locked and documented
- [x] Components and trust boundaries described
- [x] Compose topology defined
- [x] API surface drafted (`docs/api.md`)
- [x] Lifecycle FSM documented
- [x] Security baseline documented
- [x] Implementation proceeded through Stage 16 / 0.17.x
