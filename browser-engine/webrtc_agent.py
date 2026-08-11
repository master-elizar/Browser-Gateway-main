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


def gateway_ws_url() -> str:
    base = GATEWAY.replace("http://", "ws://").replace("https://", "wss://")
    return f"{base}/ws/sessions/{SESSION_ID}/signaling?role=agent&token={AGENT_TOKEN}"


async def run() -> None:
    try:
        import numpy as np
        from aiortc import RTCPeerConnection, RTCSessionDescription, VideoStreamTrack
        from av import VideoFrame
        from mss import mss
        import websockets
    except Exception as exc:  # noqa: BLE001
        print(f"[webrtc] deps missing, disabled: {exc}", flush=True)
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

    pc: RTCPeerConnection | None = None

    async def ensure_pc() -> RTCPeerConnection:
        nonlocal pc
        if pc is not None:
            return pc
        pc = RTCPeerConnection()
        pc.addTrack(X11Track())

        @pc.on("connectionstatechange")
        async def on_state() -> None:
            print(f"[webrtc] state={pc.connectionState}", flush=True)

        @pc.on("icecandidate")
        async def on_ice(candidate) -> None:  # type: ignore[no-untyped-def]
            if candidate is None or ws_holder["ws"] is None:
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
            except Exception as exc:  # noqa: BLE001
                print(f"[webrtc] ice send: {exc}", flush=True)

        return pc

    ws_holder: dict = {"ws": None}
    url = gateway_ws_url()
    print(f"[webrtc] connecting signaling {url}", flush=True)
    while True:
        try:
            async with websockets.connect(url, open_timeout=10, max_size=8 * 1024 * 1024) as ws:
                ws_holder["ws"] = ws
                print("[webrtc] signaling connected", flush=True)
                async for raw in ws:
                    try:
                        msg = json.loads(raw)
                    except json.JSONDecodeError:
                        continue
                    typ = msg.get("type")
                    if typ == "offer":
                        peer = await ensure_pc()
                        offer = RTCSessionDescription(sdp=msg["sdp"], type="offer")
                        await peer.setRemoteDescription(offer)
                        answer = await peer.createAnswer()
                        await peer.setLocalDescription(answer)
                        await ws.send(
                            json.dumps({"type": "answer", "sdp": peer.localDescription.sdp})
                        )
                    elif typ == "ice" and msg.get("candidate"):
                        peer = await ensure_pc()
                        c = msg["candidate"]
                        try:
                            await peer.addIceCandidate(
                                {
                                    "sdpMid": c.get("sdpMid"),
                                    "sdpMLineIndex": c.get("sdpMLineIndex"),
                                    "candidate": c.get("candidate"),
                                }
                            )
                        except Exception as exc:  # noqa: BLE001
                            print(f"[webrtc] addIce: {exc}", flush=True)
                    elif typ == "ready":
                        continue
        except Exception as exc:  # noqa: BLE001
            print(f"[webrtc] loop: {exc}", flush=True)
            traceback.print_exc()
            pc = None
            ws_holder["ws"] = None
            await asyncio.sleep(2)


if __name__ == "__main__":
    # reduce unused import warn
    _ = fractions
    asyncio.run(run())
