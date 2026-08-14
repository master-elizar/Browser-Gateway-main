#!/usr/bin/env bash
# Build a binary .deb that installs Browser Gateway under /opt/browser-gateway.
# Run on Linux with dpkg-deb, or on any host with Docker:
#   ./scripts/build-deb.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${BG_DEB_VERSION:-1.0.0}"
ARCH="${BG_DEB_ARCH:-amd64}"
OUT_DIR="${BG_DEB_OUT:-$ROOT/dist}"
STAGE="$OUT_DIR/browser-gateway_${VERSION}_${ARCH}"
DEB="$OUT_DIR/browser-gateway_${VERSION}_${ARCH}.deb"

rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" \
  "$STAGE/opt/browser-gateway" \
  "$STAGE/etc/systemd/system"

# Payload (exclude local secrets, git, build artifacts)
rsync -a \
  --exclude '.git' \
  --exclude '.DS_Store' \
  --exclude '**/node_modules' \
  --exclude 'backend/bin' \
  --exclude 'frontend/dist' \
  --exclude 'dist' \
  --exclude 'configs/backend.env' \
  --exclude 'deploy/.env' \
  --exclude 'deploy/traefik/certs/*.pem' \
  --exclude 'deploy/traefik/certs/*.key' \
  --exclude 'admin-credentials' \
  "$ROOT/" "$STAGE/opt/browser-gateway/"

install -m 0644 "$ROOT/packaging/debian/control" "$STAGE/DEBIAN/control"
# Refresh version in control
sed -i.bak "s/^Version:.*/Version: ${VERSION}/" "$STAGE/DEBIAN/control" && rm -f "$STAGE/DEBIAN/control.bak"
install -m 0755 "$ROOT/packaging/debian/postinst" "$STAGE/DEBIAN/postinst"
install -m 0755 "$ROOT/packaging/debian/prerm" "$STAGE/DEBIAN/prerm"
install -m 0644 "$ROOT/packaging/systemd/browser-gateway.service" "$STAGE/etc/systemd/system/browser-gateway.service"
chmod 0755 "$STAGE/opt/browser-gateway/scripts/"*.sh

mkdir -p "$OUT_DIR"
if command -v dpkg-deb >/dev/null 2>&1; then
  dpkg-deb --build "$STAGE" "$DEB"
else
  echo "==> dpkg-deb not found; building via Docker"
  docker run --rm -v "$OUT_DIR:/out" -w /out debian:bookworm-slim \
    bash -lc "apt-get update -qq && apt-get install -y -qq dpkg-dev >/dev/null && dpkg-deb --build /out/$(basename "$STAGE") /out/$(basename "$DEB")"
fi

echo "built $DEB"
ls -lh "$DEB"
