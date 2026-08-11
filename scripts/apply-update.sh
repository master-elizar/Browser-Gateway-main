#!/usr/bin/env bash
# Host-side updater triggered when data/update.requested appears.
set -euo pipefail
ROOT="${BG_INSTALL_DIR:-/opt/browser-gateway}"
MARKER="${ROOT}/data/update.requested"
PROGRESS="${ROOT}/data/update.progress"
LOG="${ROOT}/data/update.log"
mkdir -p "$ROOT/data"
exec >>"$LOG" 2>&1

# Write machine-readable progress for the Admin Updates UI.
# Usage: set_progress <percent> <phase> <message>
set_progress() {
  local pct="$1" phase="$2" msg="$3"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # Keep message JSON-safe (no quotes/newlines in phase labels we pass).
  printf '{"percent":%s,"phase":"%s","message":"%s","updatedAt":"%s","done":false}\n' \
    "$pct" "$phase" "$msg" "$ts" > "$PROGRESS"
  echo "[progress] ${pct}% ${phase}: ${msg}"
}

# Always drop the marker so a failed apply cannot permanently lock the UI.
cleanup() {
  local ec=$?
  rm -f "$MARKER"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [[ $ec -eq 0 ]]; then
    printf '{"percent":100,"phase":"complete","message":"Update finished","updatedAt":"%s","done":true}\n' \
      "$ts" > "$PROGRESS"
  else
    printf '{"percent":0,"phase":"failed","message":"Update failed (exit %s) — see update.log","updatedAt":"%s","done":true,"error":"exit %s"}\n' \
      "$ec" "$ts" "$ec" > "$PROGRESS"
  fi
  echo "==== $(date -Is) update finished exit=${ec} ===="
  exit "$ec"
}
trap cleanup EXIT

echo "==== $(date -Is) update start ===="
set_progress 5 queued "Host apply started"

cd "$ROOT"
if [[ -d .git ]]; then
  set_progress 15 fetch "Fetching origin/main"
  git fetch --depth 1 origin main
  set_progress 25 checkout "Checking out latest main"
  git checkout -q FETCH_HEAD
  echo "checkout=$(git rev-parse HEAD)"
else
  echo "WARN: no .git directory under ${ROOT}; skipping source pull"
  set_progress 25 checkout "No git dir — skipping pull"
fi

set_progress 30 prepare "Fixing data permissions"
chown -R 65532:65532 "$ROOT/data" 2>/dev/null || true
chmod 0775 "$ROOT/data" "$ROOT/data/certs" 2>/dev/null || true
chmod 0664 "$PROGRESS" 2>/dev/null || true

cd "$ROOT/deploy"
set_progress 45 build "Building stack images"
docker compose --env-file .env build

set_progress 70 engines "Building Chromium and Firefox engines"
docker compose --env-file .env --profile build-only build browser-engine browser-engine-firefox

# Only mark the tip as installed after images build successfully — otherwise the
# Admin UI thinks we are current while containers still run the previous release.
if [[ -d "$ROOT/.git" ]]; then
  git -C "$ROOT" rev-parse HEAD > "$ROOT/data/installed.commit"
  chmod 0664 "$ROOT/data/installed.commit" 2>/dev/null || true
  echo "installed.commit=$(cat "$ROOT/data/installed.commit")"
fi

set_progress 90 restart "Restarting browser-gateway service"
systemctl restart browser-gateway.service
# cleanup trap marks complete=100 after restart returns
