# Stage 14 — Multi-provider TI check

## Goal

One **Check** in the session Network panel queries every threat-intelligence source enabled in Settings → Integrations, in parallel, and shows an aggregated verdict.

## Providers

| ID | API key | Indicators |
|----|---------|------------|
| `virustotal` | required | domain, ip, url |
| `urlhaus` | none | domain, ip, url |
| `threatfox` | optional Auth-Key | domain, ip, url |
| `abuseipdb` | required | ip only |
| `otx` | required | domain, ip, url |
| `spamhaus` | none (DNSBL) | domain, ip |

## Settings

- `tiEnabled` — master switch
- `tiAutoEnrich` — optional live enrichment
- Per-provider `ti*Enabled` + API keys (masked on read)
- Legacy `tiApiKey` remains the VirusTotal key

## API

`POST /api/ti/lookup` and enrich return:

```json
{
  "provider": "multi",
  "verdict": "malicious",
  "malicious": 2,
  "suspicious": 0,
  "harmless": 3,
  "undetected": 0,
  "providers": [ { "provider": "urlhaus", "verdict": "malicious", ... } ]
}
```

Counts are **source tallies** (how many enabled providers returned each verdict), not VirusTotal engine counts.

## UI

- Badge: `MALICIOUS 2/5` (hits / sources that answered)
- Tooltip: `virustotal:clean · urlhaus:malicious · …`
- Check / recheck always fans out to all currently enabled providers
