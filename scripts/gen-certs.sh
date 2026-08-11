#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CERT_DIR="${BG_CERTS_DIR:-$ROOT/data/certs}"
mkdir -p "$CERT_DIR"
# Legacy path used by older installs — migrate once, never overwrite newer data/.
LEGACY="$ROOT/deploy/traefik/certs"
if [[ ! -f "$CERT_DIR/cert.pem" || ! -f "$CERT_DIR/key.pem" ]]; then
  if [[ -f "$LEGACY/cert.pem" && -f "$LEGACY/key.pem" ]]; then
    cp -a "$LEGACY/cert.pem" "$CERT_DIR/cert.pem"
    cp -a "$LEGACY/key.pem" "$CERT_DIR/key.pem"
    [[ -f "$LEGACY/chain.pem" ]] && cp -a "$LEGACY/chain.pem" "$CERT_DIR/chain.pem" || true
    echo "migrated certs from $LEGACY → $CERT_DIR"
  fi
fi
if [[ -f "$CERT_DIR/cert.pem" && -f "$CERT_DIR/key.pem" ]]; then
  echo "certs already exist: $CERT_DIR"
  exit 0
fi
openssl req -x509 -nodes -newkey rsa:2048 -days 825 \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -subj "/CN=browser-gateway.local/O=Browser Gateway/C=XX" \
  -addext "subjectAltName=DNS:browser-gateway.local,DNS:localhost,IP:127.0.0.1"
chmod 600 "$CERT_DIR/key.pem"
chmod 644 "$CERT_DIR/cert.pem"
echo "wrote self-signed certs to $CERT_DIR"
