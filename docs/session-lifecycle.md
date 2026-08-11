# Session Lifecycle

## States

```
CREATING → STARTING → RUNNING ⇄ IDLE → STOPPING → DESTROYED
```

Terminal state: `DESTROYED`.

## Transitions

| From | To | Trigger |
|------|----|---------|
| — | `CREATING` | `POST /browser/create` |
| `CREATING` | `STARTING` | Container created; start issued |
| `STARTING` | `RUNNING` | Health: Chromium CDP + webrtc-agent ready |
| `STARTING` | `STOPPING` | Boot timeout / crash |
| `RUNNING` | `IDLE` | No viewer connected for `idleTimeoutSec` |
| `IDLE` | `RUNNING` | Viewer reconnects |
| `RUNNING`/`IDLE` | `STOPPING` | User/admin stop, max duration, error |
| `STOPPING` | `DESTROYED` | Container removed, volumes wiped, row finalized |

## Orchestrator steps

### Create + Start

1. Admission control: global + per-user session caps from settings.
2. Insert `browser_sessions` (`CREATING`).
3. Audit: `session.creating`.
4. `docker create` from `browser-engine` image with:
   - unique name `browser-session-{id}`
   - env: `SESSION_ID`, `AGENT_TOKEN`, `GATEWAY_INTERNAL_URL`, optional `START_URL`
   - labels: `bg.session_id`, `bg.owner_id`
   - resources: memory/cpu limits from settings
5. `docker start` → `STARTING`.
6. Poll health endpoint / agent register until ready or timeout.
7. Mark `RUNNING`; audit `session.running`.

### Stop + Destroy

1. Mark `STOPPING`; audit.
2. Signal agent to drain (optional).
3. `docker stop` (timeout) → `docker rm -f`.
4. Delete named volumes / tmpfs data.
5. Mark `DESTROYED`; clear Redis runtime keys.
6. Audit `session.destroyed` with duration.

## Failure handling

- Boot timeout → force destroy + audit `session.failed` with reason.
- Agent disconnect while `RUNNING` → grace period, then `STOPPING` if unrestored.
- Backend restart: reconcile — list containers with `bg.session_id` label vs DB; adopt or destroy orphans.

## Idle & duration policies

- `idleTimeoutSec`: no WS/WebRTC viewer → `IDLE`, then stop after grace (configurable; v1 may stop directly from idle).
- `maxSessionDurationSec`: hard stop.

## Data retention vs session wipe

- Browser profile / container filesystem: **always wiped** on destroy (temporary model).
- Audit + netmon events: kept per settings and retention budget, independent of container wipe.
- Session history frames (`data/history/{sessionId}/` + `session_timeline_events`): kept for closed (`DESTROYED`) sessions; purged by `historyRetentionDays` and/or disk budget; SUPER_ADMIN can delete a session’s history.
- The `DESTROYED` DB row remains so History can list the session until retention/admin delete.
