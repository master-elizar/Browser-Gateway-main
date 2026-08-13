#!/usr/bin/env python3
"""WebRTC screen share via aiortc (X11 capture) + gateway signaling."""
from __future__ import annotations

import asyncio
import fractions
import json
import os
import time
import traceback

SESSION_ID = os.environ.get("SESSION_ID", "")
AGENT_TOKEN = os.environ.get("AGENT_TOKEN", "")
GATEWAY = os.environ.get("GATEWAY_INTERNAL_URL", "http://backend:8080").rstrip("/")
DISPLAY = os.environ.get("DISPLAY", ":99")
WIDTH = int(os.environ.get("WEBRTC_WIDTH", "1280"))
HEIGHT = int(os.environ.get("WEBRTC_HEIGHT", "800"))
FPS = int(os.environ.get("WEBRTC_FPS", "10"))
TURN_URLS = os.environ.get("TURN_URLS", "")
TURN_USERNAME = os.environ.get("TURN_USERNAME", "")
TURN_PASSWORD = os.environ.get("TURN_PASSWORD", "")


def gateway_ws_url() -> str:
    base = GATEWAY.replace("http://", "ws://").replace("https://", "wss://")
    return f"{base}/ws/sessions/{SESSION_ID}/signaling?role=agent&token={AGENT_TOKEN}"


def log(msg: str) -> None:
    # No timestamps on any of this file's prints previously -- made it impossible to tell
    # whether repeated offer/answer cycles were seconds or milliseconds apart.
    print(f"[{time.time():.3f}] [webrtc] {msg}", flush=True)


def pc_tag(peer) -> str:  # type: ignore[no-untyped-def]
    # Short identity tag so logs can show whether the SAME RTCPeerConnection is being
    # reused across offer cycles or a fresh one was created each time.
    return f"pc#{id(peer) & 0xFFFF:04x}"


async def run() -> None:
    try:
        import numpy as np
        from aiortc import (
            RTCConfiguration,
            RTCIceServer,
            RTCPeerConnection,
            RTCSessionDescription,
            VideoStreamTrack,
        )
        from aiortc.sdp import candidate_from_sdp
        from av import VideoFrame
        from mss import mss
        import websockets
    except Exception as exc:  # noqa: BLE001
        # This process is launched once by entrypoint.sh and never restarted if it exits --
        # if this fires, WebRTC is silently dead for the rest of the container's life.
        log(f"FATAL: dependency import failed, WebRTC disabled for this session: {exc!r}")
        traceback.print_exc()
        return

    class X11Track(VideoStreamTrack):
        kind = "video"

        def __init__(self) -> None:
            super().__init__()
            self._sct = mss()
            # X11 display via Xvfb is usually the first monitor under mss when DISPLAY set
            self._mon = {"left": 0, "top": 0, "width": WIDTH, "height": HEIGHT}
            self._start = time.time()

        async def recv(self):
            pts, time_base = await self.next_timestamp()
            img = self._sct.grab(self._mon)
            arr = np.frombuffer(img.bgra, dtype=np.uint8).reshape((img.height, img.width, 4))
            rgb = arr[:, :, :3][:, :, ::-1].copy()
            if rgb.shape[0] != HEIGHT or rgb.shape[1] != WIDTH:
                # nearest pad/crop
                out = np.zeros((HEIGHT, WIDTH, 3), dtype=np.uint8)
                h = min(HEIGHT, rgb.shape[0])
                w = min(WIDTH, rgb.shape[1])
                out[:h, :w] = rgb[:h, :w]
                rgb = out
            frame = VideoFrame.from_ndarray(rgb, format="rgb24")
            frame.pts = pts
            frame.time_base = time_base
            return frame

    def ice_configuration() -> RTCConfiguration:
        # Without this, this side of the connection never gathers anything but a host
        # candidate on this container's internal browser-net address -- unroutable from
        # anywhere except the Docker host itself. Same TURN server/credentials the viewer's
        # browser gets from GET /api/webrtc/ice, so both sides can actually meet on it.
        urls = [u.strip() for u in TURN_URLS.split(",") if u.strip()]
        if not urls:
            log("no TURN_URLS configured for the agent side -- only host candidates")
            return RTCConfiguration(iceServers=[])
        return RTCConfiguration(
            iceServers=[RTCIceServer(urls=urls, username=TURN_USERNAME, credential=TURN_PASSWORD)]
        )

    pc: RTCPeerConnection | None = None

    async def ensure_pc() -> RTCPeerConnection:
        nonlocal pc
        # A stale closed/failed pc must not be handed back -- a fresh offer after a prior
        # connection died (viewer navigated away, network drop, ...) is a normal renegotiation
        # from scratch, not an error condition; reusing the dead pc crashed the whole run()
        # loop with "Cannot handle offer in signaling state closed" / "RTCPeerConnection is
        # closed" instead of just handling the new offer.
        if pc is not None and pc.connectionState not in ("closed", "failed"):
            log(f"ensure_pc: reusing {pc_tag(pc)} (state={pc.connectionState})")
            return pc
        prior = pc_tag(pc) if pc is not None else None
        pc = RTCPeerConnection(configuration=ice_configuration())
        pc.addTrack(X11Track())
        log(f"ensure_pc: created {pc_tag(pc)}" + (f" (prior {prior} was stale)" if prior else " (first pc)"))

        @pc.on("connectionstatechange")
        async def on_state() -> None:
            log(f"{pc_tag(pc)} state={pc.connectionState} ice={pc.iceConnectionState}")

        @pc.on("icecandidate")
        async def on_ice(candidate) -> None:  # type: ignore[no-untyped-def]
            if candidate is None:
                log(f"{pc_tag(pc)} local ICE gathering complete")
                return
            if ws_holder["ws"] is None:
                log(f"{pc_tag(pc)} local ICE candidate ready but signaling is not connected, dropped")
                return
            try:
                await ws_holder["ws"].send(
                    json.dumps(
                        {
                            "type": "ice",
                            "candidate": {
                                "candidate": candidate.to_sdp() if hasattr(candidate, "to_sdp") else str(candidate),
                                "sdpMid": candidate.sdpMid,
                                "sdpMLineIndex": candidate.sdpMLineIndex,
                            },
                        }
                    )
                )
                log(f"{pc_tag(pc)} local ICE candidate sent")
            except Exception as exc:  # noqa: BLE001
                log(f"{pc_tag(pc)} ice send: {exc}")

        return pc

    ws_holder: dict = {"ws": None}
    url = gateway_ws_url()
    log(f"connecting signaling {url}")
    while True:
        try:
            async with websockets.connect(url, open_timeout=10, max_size=8 * 1024 * 1024) as ws:
                ws_holder["ws"] = ws
                log("signaling connected")
                async for raw in ws:
                    try:
                        msg = json.loads(raw)
                    except json.JSONDecodeError:
                        continue
                    typ = msg.get("type")
                    if typ == "offer":
                        log(f"offer received (sdp len={len(msg.get('sdp') or '')})")
                        peer = await ensure_pc()
                        offer = RTCSessionDescription(sdp=msg["sdp"], type="offer")
                        await peer.setRemoteDescription(offer)
                        answer = await peer.createAnswer()
                        await peer.setLocalDescription(answer)
                        await ws.send(
                            json.dumps({"type": "answer", "sdp": peer.localDescription.sdp})
                        )
                        log(f"{pc_tag(peer)} answer sent")
                    elif typ == "ice" and msg.get("candidate"):
                        peer = await ensure_pc()
                        log(f"{pc_tag(peer)} remote ICE candidate received")
                        c = msg["candidate"]
                        try:
                            # aiortc.addIceCandidate() takes an RTCIceCandidate object (attribute
                            # access), not the plain dict the browser's WebRTC API sends -- that
                            # mismatch ("'dict' object has no attribute 'sdpMid'") was silently
                            # dropping every single candidate, so ICE could never complete even
                            # though the SDP offer/answer exchange itself was working fine.
                            raw_sdp = (c.get("candidate") or "").strip()
                            if not raw_sdp:
                                log(f"{pc_tag(peer)} addIce: empty candidate string, skipped")
                            else:
                                body = raw_sdp[len("candidate:"):] if raw_sdp.startswith("candidate:") else raw_sdp
                                candidate = candidate_from_sdp(body)
                                candidate.sdpMid = c.get("sdpMid")
                                candidate.sdpMLineIndex = c.get("sdpMLineIndex")
                                await peer.addIceCandidate(candidate)
                                log(f"{pc_tag(peer)} remote ICE candidate added")
                        except Exception as exc:  # noqa: BLE001
                            log(f"{pc_tag(peer)} addIce: {exc}")
                    elif typ == "ready":
                        log("signaling hub acked registration (role=agent)")
                        continue
                    elif typ == "peer-missing":
                        # The hub drops (does not queue) whatever message triggered this --
                        # if it was our answer/ICE candidate, the viewer never received it and
                        # this side has no retry for its own outgoing messages the way the
                        # viewer's scheduleOfferRetry does for its offer.
                        log("signaling hub: no viewer connected yet for this session -- last outgoing message was dropped, not queued")
                        continue
                    else:
                        log(f"unhandled signaling message type={typ!r}")
        except Exception as exc:  # noqa: BLE001
            log(f"loop: {exc}")
            traceback.print_exc()
            pc = None
            ws_holder["ws"] = None
            await asyncio.sleep(2)


if __name__ == "__main__":
    # reduce unused import warn
    _ = fractions
    asyncio.run(run())
