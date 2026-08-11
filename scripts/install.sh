#!/usr/bin/env bash
# Install Browser Gateway on a bare Ubuntu/Debian host into /opt/browser-gateway.
#
#   curl -fsSL https://raw.githubusercontent.com/TaskMaster329/Browser-Gateway/main/scripts/install.sh | sudo bash
#
# Env:
#   BG_INSTALL_DIR BG_REPO_URL BG_REPO_REF BG_PUBLIC_HOST BG_SKIP_CLONE
#   BG_ADMIN_EMAIL BG_ADMIN_PASSWORD
#   BG_VERBOSE=1   show under-the-hood logs
set -euo pipefail

REPO_URL="${BG_REPO_URL:-https://github.com/TaskMaster329/Browser-Gateway.git}"
REPO_REF="${BG_REPO_REF:-main}"
INSTALL_DIR="${BG_INSTALL_DIR:-/opt/browser-gateway}"
PUBLIC_HOST="${BG_PUBLIC_HOST:-}"
ADMIN_EMAIL="${BG_ADMIN_EMAIL:-admin@browser-gateway.local}"
VERBOSE="${BG_VERBOSE:-0}"
LOG_FILE="${BG_INSTALL_LOG:-/tmp/browser-gateway-install.log}"

STEP=0
TOTAL_STEPS=9
SPINNER_PID=""

# ── UI ──────────────────────────────────────────────────────────────────────

if [[ -t 1 ]] && [[ "${TERM:-}" != "dumb" ]]; then
  C_RESET=$'\033[0m'
  C_DIM=$'\033[2m'
  C_BOLD=$'\033[1m'
  C_GREEN=$'\033[32m'
  C_CYAN=$'\033[36m'
  C_RED=$'\033[31m'
  C_YELLOW=$'\033[33m'
else
  C_RESET='' C_DIM='' C_BOLD='' C_GREEN='' C_CYAN='' C_RED='' C_YELLOW=''
fi

banner() {
  echo
  echo "${C_CYAN}${C_BOLD}  ╭──────────────────────────────────────────╮${C_RESET}"
  echo "${C_CYAN}${C_BOLD}  │         Browser Gateway Installer        │${C_RESET}"
  echo "${C_CYAN}${C_BOLD}  ╰──────────────────────────────────────────╯${C_RESET}"
  echo
  echo "  ${C_DIM}Quiet install · details → ${LOG_FILE}${C_RESET}"
  echo
}

die() {
  stop_spinner
  echo
  echo "  ${C_RED}${C_BOLD}✗  $1${C_RESET}" >&2
  if [[ -f "$LOG_FILE" ]]; then
    echo "  ${C_DIM}Last log lines:${C_RESET}" >&2
    tail -n 20 "$LOG_FILE" 2>/dev/null | sed 's/^/    /' >&2 || true
    echo "  ${C_DIM}Full log: ${LOG_FILE}${C_RESET}" >&2
  fi
  echo
  exit 1
}

stop_spinner() {
  if [[ -n "${SPINNER_PID}" ]] && kill -0 "$SPINNER_PID" 2>/dev/null; then
    kill "$SPINNER_PID" 2>/dev/null || true
    wait "$SPINNER_PID" 2>/dev/null || true
  fi
  SPINNER_PID=""
  # clear spinner line remnant
  printf '\r\033[K' 2>/dev/null || true
}

spin_while() {
  local msg="$1"
  local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
  local i=0
  while true; do
    printf '\r  %s%s%s  %s%s%s' "$C_CYAN" "${frames[i]}" "$C_RESET" "$C_DIM" "$msg" "$C_RESET"
    i=$(( (i + 1) % ${#frames[@]} ))
    sleep 0.08
  done
}

begin_step() {
  STEP=$((STEP + 1))
  local msg="$1"
  stop_spinner
  if [[ "$VERBOSE" == "1" ]]; then
    echo "  ${C_CYAN}…${C_RESET}  [${STEP}/${TOTAL_STEPS}] ${msg}"
  else
    spin_while "[${STEP}/${TOTAL_STEPS}] ${msg}" &
    SPINNER_PID=$!
    disown "$SPINNER_PID" 2>/dev/null || true
  fi
}

ok_step() {
  local msg="${1:-}"
  stop_spinner
  if [[ -n "$msg" ]]; then
    echo "  ${C_GREEN}✓${C_RESET}  [${STEP}/${TOTAL_STEPS}] ${msg}"
  else
    # reprint last known — caller should pass label
    echo "  ${C_GREEN}✓${C_RESET}  [${STEP}/${TOTAL_STEPS}] done"
  fi
}

# Run command quietly; on failure dump log and die with message.
run_quiet() {
  local fail_msg="$1"
  shift
  {
    echo
    echo "──── $(date -Is) :: $*"
    if [[ "$VERBOSE" == "1" ]]; then
      "$@"
    else
      "$@" >>"$LOG_FILE" 2>&1
    fi
  } || die "$fail_msg"
}

# ── helpers ─────────────────────────────────────────────────────────────────

need_root() {
  [[ "${EUID}" -eq 0 ]] || die "Run as root: sudo bash"
}

detect_public_host() {
  if [[ -n "$PUBLIC_HOST" ]]; then
    echo "$PUBLIC_HOST"
    return
  fi
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')" || true
  if [[ -z "$ip" ]]; then
    ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')" || true
  fi
  echo "${ip:-localhost}"
}

rand_secret() { openssl rand -hex 24; }
rand_password() { openssl rand -base64 18 | tr -d '/+=' | head -c 20; }

# ── steps ───────────────────────────────────────────────────────────────────

step_prereqs() {
  begin_step "Checking system"
  [[ -f /etc/os-release ]] || die "Unsupported OS (need Ubuntu/Debian)"
  # shellcheck source=/dev/null
  . /etc/os-release
  case "${ID:-}" in
    ubuntu|debian) ;;
    *) die "Unsupported distro '${ID:-unknown}' (need Ubuntu/Debian)" ;;
  esac
  : >"$LOG_FILE"
  chmod 600 "$LOG_FILE" 2>/dev/null || true
  ok_step "System ready (${ID} ${VERSION_CODENAME:-})"
}

step_docker() {
  begin_step "Installing Docker"
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    ok_step "Docker already installed"
    return
  fi
  # shellcheck source=/dev/null
  . /etc/os-release
  export DEBIAN_FRONTEND=noninteractive
  run_quiet "Failed to update package index" apt-get update -y
  run_quiet "Failed to install prerequisites" apt-get install -y ca-certificates curl gnupg git openssl
  install -m 0755 -d /etc/apt/keyrings
  if [[ ! -f /etc/apt/keyrings/docker.asc ]]; then
    run_quiet "Failed to download Docker GPG key" \
      curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
  fi
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  run_quiet "Failed to refresh Docker apt repo" apt-get update -y
  run_quiet "Failed to install Docker Engine" \
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  run_quiet "Failed to start Docker" systemctl enable --now docker
  ok_step "Docker Engine installed"
}

step_sources() {
  begin_step "Downloading Browser Gateway"
  if [[ "${BG_SKIP_CLONE:-0}" == "1" ]]; then
    [[ -d "$INSTALL_DIR/deploy" ]] || die "BG_SKIP_CLONE=1 but ${INSTALL_DIR}/deploy is missing"
    ok_step "Using existing files"
    return
  fi
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    run_quiet "Failed to update repository" \
      bash -c "git -C '$INSTALL_DIR' fetch --depth 1 origin '$REPO_REF' && git -C '$INSTALL_DIR' checkout -q FETCH_HEAD"
    ok_step "Updated to ${REPO_REF}"
  else
    mkdir -p "$(dirname "$INSTALL_DIR")"
    local preserve=""
    if [[ -d "$INSTALL_DIR/data" ]]; then
      preserve="$(mktemp -d /tmp/bg-data.XXXXXX)"
      mv "$INSTALL_DIR/data" "$preserve/data"
    fi
    rm -rf "$INSTALL_DIR"
    run_quiet "Failed to download repository" \
      git clone --depth 1 --branch "$REPO_REF" "$REPO_URL" "$INSTALL_DIR"
    if [[ -n "$preserve" && -d "$preserve/data" ]]; then
      rm -rf "$INSTALL_DIR/data"
      mv "$preserve/data" "$INSTALL_DIR/data"
      rm -rf "$preserve"
    fi
    ok_step "Downloaded (${REPO_REF})"
  fi
  mkdir -p "$INSTALL_DIR/data/certs"
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    git -C "$INSTALL_DIR" rev-parse HEAD > "$INSTALL_DIR/data/installed.commit" || true
    chmod 0664 "$INSTALL_DIR/data/installed.commit" 2>/dev/null || true
  fi
  # distroless nonroot is uid 65532 — must write certs from the API
  chown -R 65532:65532 "$INSTALL_DIR/data" 2>/dev/null || true
  chmod 0775 "$INSTALL_DIR/data" "$INSTALL_DIR/data/certs" 2>/dev/null || true
}
step_config() {
  local host="$1"
  begin_step "Writing configuration"
  local f="$INSTALL_DIR/configs/backend.env"
  local example="$INSTALL_DIR/configs/backend.env.example"
  [[ -f "$example" ]] || die "Missing ${example}"

  mkdir -p "$INSTALL_DIR/data/certs"
  local setup_key=""
  if [[ ! -f "$f" ]]; then
    umask 077
    cp "$example" "$f"
    sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$(rand_secret)|" "$f"
    sed -i "s|^INTERNAL_AGENT_SECRET=.*|INTERNAL_AGENT_SECRET=$(rand_secret)|" "$f"
    # First-run uses a one-time setup key (no pre-created admin password).
    sed -i "s|^BOOTSTRAP_ADMIN_EMAIL=.*|BOOTSTRAP_ADMIN_EMAIL=|" "$f"
    sed -i "s|^BOOTSTRAP_ADMIN_PASSWORD=.*|BOOTSTRAP_ADMIN_PASSWORD=|" "$f"
    setup_key="$(rand_secret)"
    printf '%s\n' "$setup_key" > "$INSTALL_DIR/data/setup.bootstrap"
    chmod 640 "$INSTALL_DIR/data/setup.bootstrap"
    chmod 600 "$f"
  fi

  local ef="$INSTALL_DIR/deploy/.env"
  local docker_gid
  docker_gid="$(getent group docker | cut -d: -f3 || true)"
  [[ -n "$docker_gid" ]] || die "Docker group not found"

  if [[ -f "$ef" ]]; then
    if grep -q '^DOCKER_GID=' "$ef"; then
      sed -i "s|^DOCKER_GID=.*|DOCKER_GID=${docker_gid}|" "$ef"
    else
      echo "DOCKER_GID=${docker_gid}" >> "$ef"
    fi
  else
    umask 077
    cat > "$ef" <<EOF
POSTGRES_PASSWORD=$(rand_password)
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=$(rand_password)
DOCKER_GID=${docker_gid}
TURN_URLS=turn:${host}:3478
GF_SERVER_ROOT_URL=http://${host}:3000
EOF
    chmod 600 "$ef"
  fi

  # Ensure API (uid 65532) can write certs / setup / update markers under data/
  chown -R 65532:65532 "$INSTALL_DIR/data" 2>/dev/null || true
  chmod 0775 "$INSTALL_DIR/data" "$INSTALL_DIR/data/certs" 2>/dev/null || true
  [[ -f "$INSTALL_DIR/data/setup.bootstrap" ]] && chmod 640 "$INSTALL_DIR/data/setup.bootstrap" || true

  if [[ -n "$setup_key" ]]; then
    echo "$setup_key" > /tmp/bg-setup-key.out
  fi
  ok_step "Configuration ready"
}

step_certs() {
  begin_step "Generating TLS certificates"
  run_quiet "Failed to generate certificates" bash "$INSTALL_DIR/scripts/gen-certs.sh"
  chown -R 65532:65532 "$INSTALL_DIR/data/certs" 2>/dev/null || true
  chmod 0775 "$INSTALL_DIR/data/certs" 2>/dev/null || true
  ok_step "Certificates ready"
}

step_service() {
  begin_step "Registering system service"
  local unit_src="$INSTALL_DIR/packaging/systemd/browser-gateway.service"
  local helper="$INSTALL_DIR/scripts/systemd-exec.sh"
  [[ -f "$helper" ]] || die "Missing ${helper}"
  chmod 0755 "$helper"
  if [[ -f "$unit_src" ]]; then
    install -m 0644 "$unit_src" /etc/systemd/system/browser-gateway.service
    install -m 0644 "$INSTALL_DIR/packaging/systemd/browser-gateway-update.service" /etc/systemd/system/browser-gateway-update.service
    install -m 0644 "$INSTALL_DIR/packaging/systemd/browser-gateway-update.path" /etc/systemd/system/browser-gateway-update.path
    chmod 0755 "$INSTALL_DIR/scripts/apply-update.sh"
    run_quiet "Failed to reload systemd" systemctl daemon-reload
    run_quiet "Failed to enable service" systemctl enable browser-gateway.service
    run_quiet "Failed to enable update watcher" systemctl enable --now browser-gateway-update.path
  fi
  ok_step "Service registered"
}

step_build_start() {
  begin_step "Building images"
  run_quiet "Failed to build images" bash "$INSTALL_DIR/scripts/systemd-exec.sh" build
  ok_step "Images built"

  begin_step "Starting Browser Gateway"
  # Single control plane: systemd owns the stack (not a bare compose up).
  if systemctl is-active --quiet browser-gateway.service 2>/dev/null; then
    run_quiet "Failed to restart service" systemctl restart browser-gateway.service
  else
    run_quiet "Failed to start service" systemctl start browser-gateway.service
  fi
  ok_step "Service started"
}

step_ready() {
  begin_step "Waiting for health check"
  local i
  for i in $(seq 1 90); do
    if curl -skf https://127.0.0.1/readyz >/dev/null 2>&1; then
      ok_step "Gateway is healthy"
      return 0
    fi
    sleep 4
  done
  stop_spinner
  echo "  ${C_YELLOW}!${C_RESET}  [${STEP}/${TOTAL_STEPS}] Health check timed out (stack may still be starting)"
  return 0
}

summary() {
  local host="$1"
  local setup_key=""
  [[ -f /tmp/bg-setup-key.out ]] && setup_key="$(cat /tmp/bg-setup-key.out)" && rm -f /tmp/bg-setup-key.out

  echo
  echo "  ${C_GREEN}${C_BOLD}╭──────────────────────────────────────────╮${C_RESET}"
  echo "  ${C_GREEN}${C_BOLD}│           Installation complete         │${C_RESET}"
  echo "  ${C_GREEN}${C_BOLD}╰──────────────────────────────────────────╯${C_RESET}"
  echo
  echo "  ${C_BOLD}URL${C_RESET}       https://${host}/"
  echo "  ${C_DIM}           open the URL and create the first admin${C_RESET}"
  echo "  ${C_BOLD}Install${C_RESET}   ${INSTALL_DIR}"
  echo "  ${C_BOLD}Certs${C_RESET}     ${INSTALL_DIR}/data/certs"
  if [[ -n "$setup_key" ]]; then
    echo
    echo "  ${C_YELLOW}${C_BOLD}ONE-TIME SETUP KEY${C_RESET}  (enter once in the web UI)"
    echo "  ${C_BOLD}${setup_key}${C_RESET}"
    echo "  ${C_DIM}shown only now — save it before closing this terminal${C_RESET}"
  fi
  echo
  echo "  ${C_BOLD}Manage${C_RESET}"
  echo "  ${C_DIM}systemctl start|stop|restart|reload|status browser-gateway${C_RESET}"
  echo
}

# ── main ────────────────────────────────────────────────────────────────────

main() {
  need_root
  banner
  trap 'stop_spinner' EXIT

  step_prereqs
  step_docker
  step_sources
  local host
  host="$(detect_public_host)"
  step_config "$host"
  step_certs
  step_service
  step_build_start
  step_ready
  summary "$host"
}

main "$@"
