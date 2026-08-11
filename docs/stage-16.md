# Stage 16 / 0.17 — Session history, downloads, paste, HTTPS

## Goals

1. **Paste → remote** inserts text into the guest browser (CDP queue + xdotool fallback).
2. **Downloads**: plain file or password-protected ZIP (admin default + dialog override).
3. **Session History**: closed sessions with screenshots (click + URL change), network, TI, audit.
4. **HTTPS only**: public HTTP removed; `:80` redirects to `:443`.

## History capture

- On control **click** (debounced) and CDP **navigation**, `session_agent` captures JPEG (~1920 wide, q~75) and POSTs to  
  `POST /internal/sessions/:id/history/frame`.
- Files: `data/history/{sessionId}/{ts}_{kind}.jpg`
- DB: `session_timeline_events`
- Cap: max frames per session (agent-side) to avoid runaway capture

## History UI / ACL

| Who | List / view | Delete |
|-----|-------------|--------|
| User | Own closed (`DESTROYED`) sessions | — |
| SUPER_ADMIN | All | Yes |

Filters: date range, name, browser, TI verdict.

Nav: **История / History** → `/history`, detail `/history/:id`.

Detail layout matches a live session:

- Main viewport shows the selected screenshot
- Horizontal **filmstrip** of frames in capture order (← → / click)
- **Network** panel is the same component as the live viewer (tabs, filters, tree, netflow) — **read-only** (no per-row TI Check; Enrich/Clear disabled)

## Retention

Admin setting `historyRetentionDays` (default **30**; **0** = no age purge). Disk budget still applies via retention worker.

## Downloads

`GET /api/browser/:id/downloads/:fileId?format=file|zip&password=`

Zip uses ZipCrypto; empty password falls back to `downloadZipPasswordDefault` (secret flag in settings, never echoed raw).

## Patch notes (0.17.x)

| Version | Change |
|---------|--------|
| **0.17.0** | History capture + APIs + UI; download ZIP; paste CDP queue |
| **0.17.1** | Frontend build fix (`PageHeader` subtitle); Update unlock after failed apply; adaptive particles |
| **0.17.2** | HTTPS only + HSTS; CSP without plaintext `ws:` |
| **0.17.3** | History detail = session chrome + horizontal screenshot filmstrip |
| **0.17.5** | Fix `SessionNetworkPanel` extract (frontend build) |
| **0.17.6** | Persist manual TI Check / Enrich onto the session so History shows the same verdicts |

## Out of scope

- Exporting full history as zip
- Screenshot every keystroke / periodic interval
- User self-delete of history
- Replaying live control into history viewer
- Per-row TI Check on closed sessions
