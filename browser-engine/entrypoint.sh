#!/bin/bash
set -euo pipefail

RESOLUTION="${RESOLUTION:-1280x800x24}"
START_URL="${START_URL:-about:blank}"
DISPLAY="${DISPLAY:-:99}"
DNS_MODE="${DNS_MODE:-docker}"
DNS_SERVERS="${DNS_SERVERS:-8.8.8.8,1.1.1.1}"
DNS_DOH_URL="${DNS_DOH_URL:-https://cloudflare-dns.com/dns-query}"
BROWSER_ENGINE="${BROWSER_ENGINE:-chromium}"
export DISPLAY HOME=/home/browser

# RESOLUTION like 1280x800x24 → WIN_W / WIN_H
WIN_W="${RESOLUTION%%x*}"
REST="${RESOLUTION#*x}"
WIN_H="${REST%%x*}"
if ! [[ "$WIN_W" =~ ^[0-9]+$ ]] || ! [[ "$WIN_H" =~ ^[0-9]+$ ]]; then
  WIN_W=1280
  WIN_H=800
fi

mkdir -p "$HOME/data" "$HOME/Downloads" "$HOME/Uploads" "$HOME/logs" /tmp/runtime-browser "$HOME/.cache" "$HOME/.config"
export XDG_RUNTIME_DIR=/tmp/runtime-browser

start_dnsmasq() {
  local servers_args=()
  IFS=',' read -ra parts <<< "$DNS_SERVERS"
  for s in "${parts[@]}"; do
    s="$(echo "$s" | xargs)"
    if [ -n "$s" ]; then
      # Never forward to Docker embedded DNS — loops when container Dns=127.0.0.1.
      if [ "$s" = "127.0.0.11" ]; then
        continue
      fi
      servers_args+=("--server=$s")
    fi
  done
  if [ ${#servers_args[@]} -eq 0 ]; then
    servers_args=(--server=8.8.8.8 --server=1.1.1.1)
  fi
  echo "[engine] dnsmasq upstream: ${servers_args[*]}"
  dnsmasq --no-daemon --log-queries --log-facility="$HOME/logs/dnsmasq.log" \
    "${servers_args[@]}" \
    --listen-address=127.0.0.1 --bind-interfaces --cache-size=1000 \
    > "$HOME/logs/dnsmasq.err" 2>&1 &
  sleep 0.3
}

CHROMIUM_EXTRA=()
FIREFOX_DOH=0

case "$DNS_MODE" in
  custom)
    echo "[engine] DNS mode=custom (dnsmasq tap)"
    start_dnsmasq
    python3 /dns_tap.py > "$HOME/logs/dns_tap.log" 2>&1 &
    ;;
  doh)
    echo "[engine] DNS mode=doh url=$DNS_DOH_URL"
    CHROMIUM_EXTRA+=(
      --dns-over-https-mode=secure
      --dns-over-https-templates="$DNS_DOH_URL"
    )
    FIREFOX_DOH=1
    ;;
  custom_doh|both)
    echo "[engine] DNS mode=custom_doh servers=$DNS_SERVERS doh=$DNS_DOH_URL"
    start_dnsmasq
    python3 /dns_tap.py > "$HOME/logs/dns_tap.log" 2>&1 &
    CHROMIUM_EXTRA+=(
      --dns-over-https-mode=secure
      --dns-over-https-templates="$DNS_DOH_URL"
    )
    FIREFOX_DOH=1
    ;;
  docker|*)
    echo "[engine] DNS mode=docker (default resolver)"
    ;;
esac

echo "[engine] starting Xvfb on $DISPLAY ($RESOLUTION)"
Xvfb "$DISPLAY" -screen 0 "$RESOLUTION" -ac +extension GLX +render -noreset \
  > "$HOME/logs/xvfb.log" 2>&1 &
sleep 0.8

echo "[engine] starting openbox"
openbox > "$HOME/logs/openbox.log" 2>&1 &
sleep 0.4

prepare_firefox_profile() {
  local profile="$HOME/data/firefox-profile"
  mkdir -p "$profile"
  # Escape backslashes and quotes for user.js string literals.
  local safe_url="${START_URL//\\/\\\\}"
  safe_url="${safe_url//\"/\\\"}"
  local safe_doh="${DNS_DOH_URL//\\/\\\\}"
  safe_doh="${safe_doh//\"/\\\"}"
  cat > "$profile/user.js" <<EOF
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("browser.startup.homepage", "$safe_url");
user_pref("browser.startup.homepage_override.mstone", "ignore");
user_pref("startup.homepage_welcome_url", "");
user_pref("startup.homepage_welcome_url.additional", "");
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
user_pref("browser.aboutwelcome.enabled", false);
user_pref("browser.tabs.warnOnClose", false);
user_pref("browser.warnOnQuit", false);
// Remote Agent: WebDriver BiDi (+ CDP when still available on older ESR)
user_pref("remote.enabled", true);
user_pref("remote.active-protocols", 3);
user_pref("devtools.chrome.enabled", true);
user_pref("devtools.debugger.remote-enabled", true);
user_pref("fission.bfcacheInParent", false);
EOF
  if [ "$FIREFOX_DOH" = "1" ]; then
    cat >> "$profile/user.js" <<EOF
user_pref("network.trr.mode", 3);
user_pref("network.trr.uri", "$safe_doh");
user_pref("network.trr.custom_uri", "$safe_doh");
EOF
  fi
  echo "$profile"
}

start_chromium() {
  echo "[engine] starting Chromium → $START_URL (dns=$DNS_MODE)"
  chromium \
    --no-sandbox \
    --disable-dev-shm-usage \
    --disable-gpu \
    --disable-software-rasterizer \
    --no-first-run \
    --no-default-browser-check \
    --disable-sync \
    --disable-translate \
    --disable-features=TranslateUI \
    --remote-debugging-address=127.0.0.1 \
    --remote-debugging-port=9222 \
    --remote-allow-origins=* \
    --user-data-dir="$HOME/data" \
    --window-size="${WIN_W},${WIN_H}" \
    --window-position=0,0 \
    --start-maximized \
    "${CHROMIUM_EXTRA[@]}" \
    "$START_URL" \
    >> "$HOME/logs/browser.log" 2>&1 &
}

start_firefox() {
  local profile
  profile="$(prepare_firefox_profile)"
  local bin="firefox-esr"
  if ! command -v firefox-esr >/dev/null 2>&1; then
    bin="firefox"
  fi
  echo "[engine] starting Firefox ($bin) → $START_URL (dns=$DNS_MODE) remote=:9222"
  # --remote-debugging-port enables WebDriver BiDi (and CDP on older ESR) for netmon.
  "$bin" \
    --no-remote \
    --profile "$profile" \
    --width "$WIN_W" \
    --height "$WIN_H" \
    --remote-debugging-port=9222 \
    "$START_URL" \
    >> "$HOME/logs/browser.log" 2>&1 &
}

start_browser() {
  case "$BROWSER_ENGINE" in
    firefox)
      start_firefox
      ;;
    chromium|*)
      start_chromium
      ;;
  esac
}

browser_running() {
  case "$BROWSER_ENGINE" in
    firefox)
      pgrep -f "firefox" >/dev/null 2>&1
      ;;
    *)
      pgrep -f "chromium" >/dev/null 2>&1
      ;;
  esac
}

: > "$HOME/logs/browser.log"
start_browser
sleep 1.5

echo "[engine] starting x11vnc"
x11vnc -display "$DISPLAY" -forever -shared -rfbport 5900 -nopw -localhost \
  > "$HOME/logs/x11vnc.log" 2>&1 &

sleep 0.5

echo "[engine] starting websockify/noVNC on :6080"
websockify --web /usr/share/novnc 6080 localhost:5900 \
  > "$HOME/logs/websockify.log" 2>&1 &

echo "[engine] starting session agent on :8090"
python3 /session_agent.py > "$HOME/logs/agent.log" 2>&1 &

echo "[engine] starting control agent on :8091"
python3 /control_agent.py > "$HOME/logs/control.log" 2>&1 &

echo "[engine] starting webrtc agent"
python3 /webrtc_agent.py > "$HOME/logs/webrtc.log" 2>&1 &

echo "[engine] ready session=${SESSION_ID:-unknown} browser=$BROWSER_ENGINE"

while true; do
  if ! kill -0 "$(pgrep -n Xvfb)" 2>/dev/null; then
    echo "[engine] Xvfb died"; exit 1
  fi
  if ! browser_running; then
    echo "[engine] $BROWSER_ENGINE died — restarting"
    start_browser
    sleep 2
  fi
  sleep 2
done
