#!/usr/bin/env bash
# Helper invoked by systemd (and install.sh) to manage the Compose stack.
# Usage: systemd-exec.sh start|stop|reload|build
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY="$ROOT/deploy"
COMPOSE=(docker compose --env-file .env)

[[ -d "$DEPLOY" ]] || { echo "missing deploy dir: $DEPLOY" >&2; exit 1; }
cd "$DEPLOY"
[[ -f .env ]] || { echo "missing $DEPLOY/.env" >&2; exit 1; }

cmd="${1:-}"
case "$cmd" in
  start)
    "${COMPOSE[@]}" up -d --remove-orphans
    ;;
  stop)
    # stop keeps containers/networks for a fast restart (do not use `down` here)
    "${COMPOSE[@]}" stop
    ;;
  reload)
    "${COMPOSE[@]}" up -d --remove-orphans
    ;;
  build)
    "${COMPOSE[@]}" build
    "${COMPOSE[@]}" --profile build-only build browser-engine browser-engine-firefox
    ;;
  *)
    echo "usage: $0 start|stop|reload|build" >&2
    exit 1
    ;;
esac
