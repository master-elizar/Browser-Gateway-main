# Browser Gateway

Self-hosted «Chrome Remote Browser as a Service»: локальный пользователь логинится,
жмёт **Launch Browser** и получает изолированный Chromium (или Firefox), поднятый
в отдельном одноразовом Docker-контейнере и транслируемый в браузер через noVNC —
плюс панель живого сетевого трафика (DNS/HTTP), проверку доменов по десятку
threat-intel источников, и историю уже закрытых сессий со скриншотами.

Вся конфигурация — Docker Compose (`deploy/docker-compose.yml`), Traefik перед
всем стеком по HTTPS. Поднять стенд можно двумя способами:

- **`scripts/install.sh`** — единственная команда на голом Ubuntu/Debian.
  Ставит Docker, скачивает репозиторий в `/opt/browser-gateway`, сама
  генерирует секреты и TLS-сертификаты, регистрирует systemd-юнит и поднимает
  весь стек. Рекомендуемый путь для реального сервера — именно так это и
  задумано (см. [Быстрый старт](#быстрый-старт)).
- **Руками через `docker compose`** — если хотите сами управлять секретами,
  systemd и обновлениями. В этом случае нужно повторить руками то, что делает
  `install.sh`: скопировать `configs/backend.env.example` →
  `configs/backend.env` и `deploy/.env.example` → `deploy/.env`, подставить
  свои значения (см. [Настройка](#настройка)), собрать образы и поднять
  `docker compose -f deploy/docker-compose.yml up -d --build`. Это ничем не
  проще первого способа, просто даёт больше контроля.

## Быстрый старт

```bash
curl -fsSL https://raw.githubusercontent.com/master-elizar/Browser-Gateway-main/master/scripts/install.sh | sudo bash
```

Это единственная команда, которая нужна на чистом Ubuntu/Debian. Она проходит
9 шагов: проверка ОС → установка Docker Engine → скачивание репозитория в
`/opt/browser-gateway` → генерация секретов (`JWT_SECRET`,
`INTERNAL_AGENT_SECRET`, пароль Postgres, пароль Grafana) и одноразового
setup-ключа → self-signed TLS-сертификат → регистрация systemd-юнитов →
сборка образов → запуск стека → ожидание `/readyz`. Первый запуск занимает
несколько минут — почти всё время уходит на сборку образов (Go-бэкенд,
Vite-фронтенд, `browser-engine` с Chromium внутри).

По завершении в терминале появится ссылка и **одноразовый setup-ключ** —
он показывается только один раз, сохраните его сразу:

```
URL       https://<host>/
ONE-TIME SETUP KEY   a1b2c3d4e5f6...
```

Откройте URL — увидите мастер первичной настройки из пяти шагов: ввод
setup-ключа и создание первого администратора → сетевой доступ (TURN/WebRTC,
можно пропустить) → лимиты сессий → имя инстанса → подтверждение. После
этого можно логиниться и жать **Launch Browser**.

Чтобы изменить адрес, на который завязан TURN, задайте его явно:

```bash
curl -fsSL https://raw.githubusercontent.com/master-elizar/Browser-Gateway-main/master/scripts/install.sh | sudo BG_PUBLIC_HOST='your.host.or.ip' bash
```

Повторный запуск того же install.sh на уже развёрнутом хосте — это не
переустановка с нуля: он обновит код (`git fetch` + `checkout`) и
перезапустит сервис, секреты и данные (Postgres/Redis volumes, `data/`)
не трогает.

## Архитектура

```
                    Пользователь (браузер)
                              │
                            HTTPS
                       (Traefik :443,
                    :80 → редирект на :443)
                              │
              ┌───────────────┴────────────────┐
              │                                 │
              ▼                                 ▼
        frontend (SPA)                   backend (Go/Fiber)
        React + Vite + Tailwind          REST + WebSocket API
                                                 │
                        ┌────────────┬───────────┼────────────┬─────────────┐
                        ▼            ▼            ▼            ▼             ▼
                  PostgreSQL       Redis    Docker Engine    coturn      threat-intel
                  (пользователи,  (кэш,     (unix-сокет,     (TURN,      провайдеры
                  сессии, аудит,  rate-     напрямую HTTP    network_    (см. ниже)
                  settings, TI-   limit)    API, без SDK)    mode: host)
                  кэш)                            │
                                                   │ создаёт динамически
                                                   ▼
                                     browser-engine (per-session контейнер)
                                     ┌─────────────────────────────────────┐
                                     │ Xvfb · Chromium/Firefox · CDP        │
                                     │ session_agent.py (netmon + control)  │
                                     │ webrtc_agent.py (сейчас на паузе)     │
                                     │ x11vnc + websockify (noVNC)          │
                                     └─────────────────────────────────────┘
```

- **frontend** — React 19 + TypeScript + Vite + Tailwind 4 SPA: список сессий,
  вьювер (поток + тулбар + сетевая панель), история, проверка доменов,
  админка (пользователи, сессии, настройки, аудит, TLS, обновления).
- **backend** — один Go-процесс (Fiber): auth, orchestrator, WebSocket-хабы
  (signaling/netmon/control), threat-intel сервис, история, аудит, настройки,
  self-update. Docker-сокет примонтирован **только** сюда — ни в один
  browser-контейнер он не попадает никогда (см. `docs/security.md`).
- **postgres / redis** — постоянные данные и runtime-состояние.
- **traefik** — единственная точка входа снаружи, только HTTPS.
- **coturn** — TURN-relay для WebRTC (`network_mode: host`, слушает все
  интерфейсы хоста).
- **browser-engine** — образ с Chromium или Firefox, инстанс поднимается
  оркестратором на каждую сессию отдельно (это **не** статичный сервис в
  Compose) и уничтожается при остановке сессии; сеть — изолированный мост
  `browser-net`, наружу не публикуется.

## Как это устроено

Сессионные контейнеры не описаны как сервисы в `docker-compose.yml` — их
создаёт `backend/internal/orchestrator` напрямую через Docker Engine HTTP API
по unix-сокету (`/var/run/docker.sock`, без docker SDK). Каждый — со своими
`privileged: false`, `cap_drop: ALL`, `read_only: true`, non-root, в сети
`browser-net`; сокет туда не пробрасывается никогда.

Стриминг сейчас — **только noVNC** (websockify, проксируется бэкендом 1:1).
WebRTC-путь (`webrtc_agent.py`, отдельный signaling-хаб, TURN через coturn)
полностью реализован и код никуда не делся, но временно отключён на уровне
UI (кнопка неактивна, с пояснением) — заново включается одним булевым флагом
на фронтенде, когда до него дойдут руки.

### Почему обновления и сетевые настройки применяются не мгновенно

Backend работает внутри контейнера и физически не может сам себе
переписать `deploy/.env` или перезапустить `docker compose` — у него просто
нет доступа к хосту снаружи Docker. Поэтому и self-update
(Admin → Settings → Updates), и применение сетевых настроек (TURN_URLS) из
setup-мастера идут через один и тот же паттерн:

1. Backend пишет маркер-файл в `data/` (общий bind-mount с хостом):
   `update.requested` или `network.requested`.
2. systemd `.path`-юнит на хосте (`browser-gateway-update.path`,
   `browser-gateway-network.path`) видит появление файла и запускает
   соответствующий oneshot `.service`.
3. Тот вызывает `scripts/apply-update.sh` / `scripts/apply-network.sh` —
   обычный bash-скрипт **на хосте**, у которого есть доступ к
   `deploy/.env` и `docker compose`. Он правит нужные переменные (с
   бэкапом) и перезапускает нужные сервисы.
4. Скрипт пишет JSON с прогрессом обратно в `data/`, backend его читает и
   отдаёт фронтенду по поллингу.

Отсюда и разница между `AppSettings` (хранится в Postgres, применяется сразу
без рестарта — большинство настроек в админке) и `.env`-параметрами вроде
`TURN_URLS` (резолвятся Compose один раз при создании контейнера, реально
меняются только через этот marker-файл механизм, с перезапуском backend).

## Что происходит при `install.sh`

1. **Проверка системы** — только Ubuntu/Debian.
2. **Docker** — если ещё не установлен, ставится Docker Engine из
   официального APT-репозитория.
3. **Скачивание** — `git clone --depth 1` в `/opt/browser-gateway` (при
   повторном запуске — `fetch` + `checkout`, данные в `data/` сохраняются).
4. **Конфигурация** — генерируются `configs/backend.env` (из `.example`, со
   случайными `JWT_SECRET`/`INTERNAL_AGENT_SECRET`) и `deploy/.env` (пароли
   Postgres/Grafana, `DOCKER_GID`, `TURN_URLS` на основе реального адреса
   хоста — определяется через `ip route get 1.1.1.1`, а не `hostname -I`,
   которая на многосетевых Docker-хостах может вернуть docker-внутренний
   адрес вместо настоящего). Плюс одноразовый setup-ключ.
5. **TLS** — self-signed сертификат (`scripts/gen-certs.sh`).
6. **systemd** — `browser-gateway.service` + `.path`-юниты для
   update/network-apply.
7. **Сборка образов** — backend, frontend, browser-engine (Chromium и
   Firefox).
8. **Запуск** — через systemd, не голым `docker compose up`.
9. **Health-check** — ждёт `/readyz` до 6 минут (90 попыток по 4 секунды).

## Порты

| Сервис | Где | Снаружи |
|---|---|---|
| Web UI (frontend + API + WS) | Traefik | `https://<host>/` (`:80` → редирект на `:443`) |
| Grafana | monitor-стек | `http://<host>:3000` |
| Prometheus | monitor-стек | `http://<host>:9090` |
| Loki | monitor-стек | `http://<host>:3100` |
| Alertmanager | monitor-стек | `http://<host>:9093` |
| TURN (coturn) | host-сеть | `<host>:3478` (UDP+TCP) |
| PostgreSQL / Redis | internal-сеть | не публикуются |
| Backend API напрямую | internal-сеть | не публикуется — только через Traefik |
| Сессионные browser-engine контейнеры | `browser-net` | не публикуются вообще |

## Структура проекта

```
.
├── backend/          # Go 1.23 (Fiber): auth, orchestrator, TI, netmon, admin API
├── frontend/          # React 19 + TS + Vite + Tailwind SPA
├── browser-engine/     # Python-агенты внутри per-session контейнера
│   ├── session_agent.py   # CDP netmon + control-канал
│   ├── webrtc_agent.py    # WebRTC-стриминг (сейчас отключён на UI)
│   ├── dns_tap.py         # DNS-перехват для DNS_MODE=custom/custom_doh
│   └── control_agent.py
├── deploy/
│   ├── docker-compose.yml
│   ├── traefik/            # dynamic.yml — маршруты, security headers
│   └── monitoring/         # Prometheus, Grafana, Loki, Alertmanager конфиги
├── configs/
│   ├── backend.env.example
│   └── ...
├── packaging/
│   ├── debian/              # .deb (control, postinst, prerm)
│   └── systemd/              # browser-gateway.service + update/network .path-юниты
├── scripts/
│   ├── install.sh            # см. "Быстрый старт"
│   ├── apply-update.sh        # host-side self-update (см. "Как это устроено")
│   ├── apply-network.sh       # host-side применение TURN_URLS
│   ├── build-deb.sh
│   └── gen-certs.sh
├── docs/               # архитектура, API, security, session lifecycle, backlog
└── tests/              # заявлено как e2e/integration, фактически пусто
```

## Настройка

Часть параметров живёт в Postgres (`AppSettings`, правится через
Admin → Settings без перезапуска), часть — в файлах, которые резолвятся
один раз при старте контейнеров:

- **`configs/backend.env`** — `JWT_SECRET`, `INTERNAL_AGENT_SECRET`,
  `MAX_SESSIONS_GLOBAL`/`MAX_SESSIONS_PER_USER`, `SESSION_IDLE_TIMEOUT`
  (по умолчанию 30 минут), `SESSION_MAX_DURATION`.
- **`deploy/.env`** — пароли Postgres/Grafana, `DOCKER_GID`, `TURN_URLS`.
  **`TURN_URLS` — не docker-внутренний адрес**, а адрес(а), на которые
  реально смотрит **браузер зрителя**: для доступа только с самого хоста
  достаточно `turn:localhost:3478`, для LAN — реальный LAN IP хоста, для
  VPN (Tailscale и т.п.) — его адрес там, для доступа из интернета —
  публичный IP/домен **плюс** проброшенные на роутере UDP+TCP 3478 (и
  relay-диапазон coturn) **плюс** раскомментированный `--external-ip` у
  `coturn` в `docker-compose.yml`. Можно перечислить несколько адресов через
  запятую — WebRTC перебирает все и использует тот, что подключился (сейчас
  неактуально, пока WebRTC на паузе, но настройка сохраняется на будущее).
- **Admin → Settings** — retention (10 GiB по умолчанию, ротация от
  старых), сроки хранения истории, что логировать (визиты URL, скачивания,
  DNS/HTTP-трафик, нажатия клавиш — по умолчанию выключено), политика
  паролей, регистрация новых пользователей, TLS-сертификат, ключи
  threat-intel провайдеров.
- **Личные API-ключи** — Account → API keys: пользователь может задать свой
  ключ для любого threat-intel провайдера, требующего ключ — используется
  вместо общего ключа проекта для его собственных проверок (свои лимиты).
- **Geo/CIDR-ограничение доступа** — сознательно не реализовано на уровне
  приложения (это была бы ложная гарантия безопасности без настоящего
  фаервола перед ним); ограничивайте доступ на уровне сети/фаервола хоста
  или reverse-proxy перед Traefik.

## Проверка доменов (threat intelligence)

Отдельная вкладка проверяет домен параллельно по всем настроенным
источникам и агрегирует в один из трёх вердиктов (безопасен / подозрителен /
вредоносен) с разбивкой по каждому источнику в расширенном режиме, плюс
WHOIS/RDAP. Источники без ключа работают из коробки, для остальных нужен
ключ (общий на проект или личный):

| Источник | Ключ | Комментарий |
|---|---|---|
| Spamhaus DNSBL | не нужен | ZEN + DBL через DNS |
| URLhaus | не нужен | abuse.ch |
| ThreatFox | опционален | abuse.ch |
| crt.sh | не нужен | Certificate Transparency, информационно |
| Feodo Tracker | не нужен | C2-блоклист abuse.ch |
| MalwareBazaar | опционален | поиск по хэшу файла |
| VirusTotal | нужен | |
| AlienVault OTX | нужен | |
| AbuseIPDB | нужен | |
| Shodan | нужен | открытые порты/баннеры, информационно |
| Google Safe Browsing | нужен | |

По завершении каждой сессии домены из её трафика автоматически проверяются
по Spamhaus + URLhaus (единственные два источника без ключа — специально,
чтобы не долбить платные/rate-limited API на весь трафик автоматически), и
итог показывается баннером в вьювере и истории.

## Troubleshooting

**WebRTC не подключается / зависает на "connecting"** — по ходу починки это
было несколько отдельных багов (трассировка SDP/ICE отсутствовала,
`addIceCandidate` ждёт объект, а не dict, пересоздание `RTCPeerConnection`
падало на уже закрытом соединении, `TURN_URLS` резолвился в
docker-внутренний адрес из-за `hostname -I`) — все исправлены, но сейчас
WebRTC намеренно отключён на UI, используется только noVNC, пока не дойдут
руки закончить проверку стабильности. Если видите неактивную кнопку WebRTC
с подсказкой "скоро" — это ожидаемо, не баг.

**Панель "Сеть" перестаёт обновляться во время активной сессии, независимо
от лимита событий** — уже исправлено, но суть бага стоит знать: noVNC-прокси
трогал "последнюю активность" сессии только один раз при подключении, а не
на каждый реальный клик/ввод. Через `SESSION_IDLE_TIMEOUT` (по умолчанию
30 минут) сессия тихо переходила в `IDLE` **несмотря на то, что ей активно
пользовались**, а фронтенд корректно отказывался переподключать
netmon-WebSocket, пока статус не `RUNNING`. Сейчас `serveVNCProxy`
(`backend/internal/handlers/stream.go`) обновляет активность по реальному
вводу с рейт-лимитом раз в 20 секунд.

**После переустановки WebRTC работает с самого хоста, но не с других
устройств в сети** — см. `TURN_URLS` в [Настройка](#настройка): это
docker-внутренний адрес, если `install.sh` определил его через `hostname -I`
на многосетевом хосте, а не через `ip route get`. Актуальный `install.sh`
это уже не делает; если стенд разворачивался старой версией скрипта —
поправьте `TURN_URLS` в `deploy/.env` руками и перезапустите backend.

**Скачивание ZIP с паролем — поле пароля выглядит без стилей** — был баг
с несуществующим CSS-классом, уже исправлен.

## Известные ограничения

- **WebRTC временно на паузе** — весь код (`webrtc_agent.py`, signaling-хаб,
  `WebRTCViewer.tsx`) на месте и не удалён, отключён только UI-путь одним
  флагом; активный способ просмотра сейчас — только noVNC.
- **Self-signed TLS по умолчанию** — своя пара сертификатов генерируется
  install.sh, ACME/Let's Encrypt автоматизации нет; можно загрузить свой
  сертификат через Admin → Settings → TLS.
- **Один хост** — оркестратор рассчитан на единственный Docker-движок,
  multi-node не реализован (интерфейс `Orchestrator` спроектирован с этим
  в уме, но масштабирование — будущая работа).
- **Нет MFA, нет IP allowlist на уровне приложения** — осознанные
  non-goals для текущей версии (см. `docs/security.md`).
- **Нет CI** — ничего не проверяет сборку/тесты автоматически при пуше.
  `backend/` содержит часть `_test.go`, но они не гоняются регулярно;
  `tests/` в корне — пустая заглушка, несмотря на описание в структуре
  проекта.
- **Нет записи видео сессий и PCAP** — сознательно вне MVP.
- **Персистентные профили браузера** — сознательно не реализованы (требуют
  отдельной модели изоляции, которой пока нет).

## Возможные направления развития

- **Вернуть WebRTC** — код готов, отключён только на UI; нужно докончить
  начатую диагностику стабильности на реальном трафике.
- **Let's Encrypt / ACME** — заменить self-signed на автоматическое
  получение настоящего сертификата.
- **MFA / OIDC** — сейчас только локальный логин с паролем.
- **Multi-node оркестрация** — сейчас один Docker-движок на весь стенд.
- **Network waterfall / timing view** — сетевая панель сейчас про факт
  запроса и его метаданные, не про тайминги загрузки.
- **Экспорт истории** — zip из кадров + дампа сетевого трафика сессии.
- **Persistent-профили браузера** — явно вне MVP, требует отдельной истории
  про изоляцию между сессиями одного профиля.
