# Stage 13 — Threat Intelligence (VirusTotal)

## Goal

Enrich session network indicators (domains, IPs, URLs) against a TI provider so analysts see reputation next to live netmon traffic.

## Provider (v1)

**VirusTotal API v3**

- Domain: `GET /api/v3/domains/{domain}`
- IP: `GET /api/v3/ip_addresses/{ip}`
- URL: `GET /api/v3/urls/{base64url(url)}` (fallback: submit + poll not required for MVP)

API key stored in `app_settings` (masked on read). Results cached in `ti_cache` (~24h).

## Settings (Integrations)

| Field | Meaning |
|-------|---------|
| `tiEnabled` | Master switch |
| `tiProvider` | `virustotal` (extensible later) |
| `tiApiKey` | VT API key (write-only / masked) |
| `tiAutoEnrich` | Lookup hosts from new netmon events (rate-limited) |

## API

- `POST /api/ti/lookup` `{ "kind": "domain"|"ip"|"url", "value": "..." }` → verdict + stats
- `POST /api/browser/:id/network/enrich` `{ "values": ["example.com", ...] }` → batch map
- Settings via existing `GET/PUT /api/admin/settings`

## UI

- Settings → Integrations → VirusTotal card
- Session Network panel: TI badge per row + Lookup / Enrich visible

## Notes

- Free VT keys are rate-limited (~4/min). Cache aggressively; auto-enrich skips when quota-ish errors occur.
- Never log the raw API key.
