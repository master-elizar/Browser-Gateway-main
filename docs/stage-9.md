# Stage 9 — Monitoring

## Delivered

- Backend `/metrics` (Prometheus; not via Traefik)
- Prometheus (`:9090`) + Grafana (`:3000`)
- **Loki** (`:3100`) + Promtail (Docker log scrape)
- **Alertmanager** (`:9093`) with basic rules
- Version field `0.9.5`

## Access

| Service | Port |
|---------|------|
| Grafana | `:3000` |
| Prometheus | `:9090` |
| Loki | `:3100` |
| Alertmanager | `:9093` |

Open as `http://<host>:<port>` on your deploy host (Grafana is bound on the host network; the main app UI is HTTPS-only on `:443`).

## Not included

- Full OTLP distributed tracing (Stage 10+)
