#!/usr/bin/env bash
# Host-side network/TURN reconfigurator triggered when data/network.requested appears.
# Mirrors apply-update.sh's marker/progress protocol -- see backend/internal/handlers/network.go.
set -euo pipefail
ROOT="${BG_INSTALL_DIR:-/opt/browser-gateway}"
MARKER="${ROOT}/data/network.requested"
PROGRESS="${ROOT}/data/network.progress"
LOG="${ROOT}/data/network.log"
ENV_FILE="${ROOT}/deploy/.env"
mkdir -p "$ROOT/data"
exec >>"$LOG" 2>&1

# Write machine-readable progress for the setup wizard / admin UI.
# Usage: set_progress <percent> <phase> <message>
set_progress() {
  local pct="$1" phase="$2" msg="$3"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
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
    printf '{"percent":100,"phase":"complete","message":"Network settings applied","updatedAt":"%s","done":true}\n' \
      "$ts" > "$PROGRESS"
  else
    printf '{"percent":0,"phase":"failed","message":"Apply failed (exit %s) — see network.log","updatedAt":"%s","done":true,"error":"exit %s"}\n' \
      "$ec" "$ts" "$ec" > "$PROGRESS"
  fi
  echo "==== $(date -Is) network apply finished exit=${ec} ===="
  exit "$ec"
}
trap cleanup EXIT

echo "==== $(date -Is) network apply start ===="
set_progress 5 queued "Host apply started"

[[ -f "$MARKER" ]] || { echo "no marker present, nothing to do"; exit 0; }

# Marker is a plain key=value list (same style as update.requested), not JSON -- keeps this
# script dependency-free (no jq).
TURN_URLS_NEW=""
while IFS='=' read -r key value; do
  case "$key" in
    turnUrls) TURN_URLS_NEW="$value" ;;
  esac
done < "$MARKER"

if [[ -z "$TURN_URLS_NEW" ]]; then
  echo "ERROR: marker had no turnUrls value"
  exit 1
fi

set_progress 25 configure "Updating deploy/.env"
[[ -f "$ENV_FILE" ]] || { echo "ERROR: missing $ENV_FILE"; exit 1; }
cp "$ENV_FILE" "${ENV_FILE}.bak-$(date +%s)"
if grep -q '^TURN_URLS=' "$ENV_FILE"; then
  sed -i "s|^TURN_URLS=.*|TURN_URLS=${TURN_URLS_NEW}|" "$ENV_FILE"
else
  echo "TURN_URLS=${TURN_URLS_NEW}" >> "$ENV_FILE"
fi
echo "TURN_URLS now: ${TURN_URLS_NEW}"

set_progress 60 recreate "Recreating backend with new settings"
cd "$ROOT/deploy"
docker compose --env-file .env up -d --remove-orphans backend

# cleanup trap marks complete=100
