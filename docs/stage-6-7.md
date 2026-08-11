# Stage 6/7 — Chromium + stream + toolbar/netmon

## Delivered

- Real **Chromium** in `browser-engine` (Xvfb + openbox)
- CDP on `127.0.0.1:9222` inside container
- Stream: **WebRTC primary** (aiortc X11 capture) + **noVNC fallback**
- Signaling WS: `/ws/sessions/{id}/signaling`
- Control WS: `/ws/sessions/{id}/control` → xdotool
- coturn (TURN) in default Compose
- Backend WS: `/ws/sessions/{id}/vnc` and `/ws/sessions/{id}/netmon`
- Session agent: CDP Network → gateway, clipboard, upload/download
- **dnsmasq DNS tap** → real DNS events with answers
- Toolbar: clipboard, upload, downloads, network panel filters

## Verify

1. Launch Browser → RUNNING → WebRTC video (or switch to noVNC)
2. Browse → Network panel shows http + dns (+ IP answers)
3. Mouse/keyboard via WebRTC control path
4. Paste → remote / Copy ← remote
5. Stop destroys container
