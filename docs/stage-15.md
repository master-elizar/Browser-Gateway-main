# Stage 15 — Network request tree

## Goal

Show how a visited page pulls other domains and requests: **page → hosts → URLs**.

## Capture

`browser-engine/session_agent.py` enriches HTTP netmon events with:

| Field | Source |
|-------|--------|
| `documentURL` | CDP `Network.requestWillBeSent.documentURL` (BiDi: Referer fallback) |
| `initiator` | `{ type, url }` from CDP initiator / stack |
| `resourceType` | CDP `type` (Document, Script, …); BiDi `destination` mapped |

## UI

Session Network tab **Tree / Дерево**:

1. Root: Document navigations (or grouped by `documentURL`)
2. Children: unique hosts requested under that page
3. Grandchildren: individual HTTP requests (expand host)

TI **Check** works on page host, child host, and each URL.

## Notes

- Flat DNS/HTTP/FQDN tabs unchanged.
- Older sessions without tree fields still group via Referer when present; otherwise land under “Unknown page”.
