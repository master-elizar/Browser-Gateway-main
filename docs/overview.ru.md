# Обзор (RU)

Актуальная линейка: **0.17.x** (Stage 16). Полный индекс: [README.md](README.md). Спеки: [architecture.md](architecture.md), [mvp-scope.md](mvp-scope.md), [api.md](api.md), [security.md](security.md).

## Что это

SaaS удалённого браузера: пользователь логинится → **Launch Browser** → видит живой **Chromium в Docker** через **WebRTC** (fallback **noVNC**). После остановки сессия доступна в **Истории** со скриншотами и тем же Network-панелем (только чтение).

## Компоненты

- **frontend** — React/TS, EN/RU, viewer, история, админка
- **backend** (Go/Fiber) — auth, sessions, orchestrator, signaling, netmon, TI, history, audit, settings, updates
- **postgres / redis** — данные и runtime
- **traefik** — только HTTPS (+ редирект с `:80`), HSTS, WS/`WSS`
- **browser-engine** — Chromium (+ Firefox образ), Xvfb, WebRTC agent, CDP, netmon, history capture
- **coturn** — TURN для WebRTC

Контейнеры сессий создаёт orchestrator динамически.

## Возможности MVP+

- Temporary-сессии (контейнер уничтожается; запись `DESTROYED` + история кадров остаются)
- Local auth: `SUPER_ADMIN` / `USER`
- Toolbar: clipboard (Paste→remote / Copy←remote), upload, download (файл / ZIP с паролем), network, stop
- Network: DNS, HTTP, URL, FQDN, IP, TI, Tree, Netflow + фильтры
- TI: несколько провайдеров (VirusTotal, URLhaus, ThreatFox, AbuseIPDB, OTX, Spamhaus)
- История: клик/навигация → JPEG; ACL владелец / админ; удаление только SUPER_ADMIN
- Админ: пользователи, сессии, настройки, TLS, retention, updates

## Жизненный цикл

`CREATING → STARTING → RUNNING ⇄ IDLE → STOPPING → DESTROYED`

## Доступ

Только `https://<host>/` (self-signed в v1). Мониторинг: Grafana `:3000`.
