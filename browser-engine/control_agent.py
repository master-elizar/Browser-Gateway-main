#!/usr/bin/env python3
"""Control websocket: pointer/keyboard → xdotool."""
from __future__ import annotations

import asyncio
import json
import os
import subprocess
import urllib.request

DISPLAY = os.environ.get("DISPLAY", ":99")
PORT = int(os.environ.get("CONTROL_PORT", "8091"))
AGENT_HTTP = os.environ.get("AGENT_HTTP", "http://127.0.0.1:8090")


def xdotool(*args: str) -> None:
    env = {**os.environ, "DISPLAY": DISPLAY}
    try:
        subprocess.run(["xdotool", *args], env=env, check=False, timeout=2)
    except Exception as exc:  # noqa: BLE001
        print(f"[control] xdotool: {exc}", flush=True)


def notify_history_click(x: int, y: int, button: int) -> None:
    try:
        data = json.dumps(
            {"kind": "click", "meta": {"x": x, "y": y, "button": button}}
        ).encode()
        req = urllib.request.Request(
            f"{AGENT_HTTP}/history/capture",
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=2) as resp:
            resp.read()
    except Exception:
        pass


def handle(msg: dict) -> None:
    typ = msg.get("type")
    if typ == "mousemove":
        x, y = int(msg.get("x", 0)), int(msg.get("y", 0))
        xdotool("mousemove", "--sync", str(x), str(y))
    elif typ == "mousedown":
        xdotool("mousedown", str(int(msg.get("button", 1))))
    elif typ == "mouseup":
        xdotool("mouseup", str(int(msg.get("button", 1))))
    elif typ == "click":
        button = int(msg.get("button", 1))
        x = int(msg.get("x", 0))
        y = int(msg.get("y", 0))
        if x or y:
            xdotool("mousemove", "--sync", str(x), str(y))
        xdotool("click", str(button))
        notify_history_click(x, y, button)
    elif typ == "keydown":
        key = str(msg.get("key") or msg.get("code") or "")
        if key:
            xdotool("key", key)
    elif typ == "type":
        text = str(msg.get("text") or "")
        if text:
            xdotool("type", "--clearmodifiers", "--", text)
    elif typ == "wheel":
        dy = int(msg.get("deltaY") or 0)
        if dy > 0:
            xdotool("click", "5")
        elif dy < 0:
            xdotool("click", "4")


async def handler(websocket) -> None:
    await websocket.send(json.dumps({"type": "ready"}))
    async for raw in websocket:
        try:
            msg = json.loads(raw)
        except json.JSONDecodeError:
            continue
        handle(msg)


async def main() -> None:
    try:
        import websockets
    except ImportError:
        print("[control] websockets missing", flush=True)
        return
    print(f"[control] listening :{PORT}", flush=True)
    async with websockets.serve(handler, "0.0.0.0", PORT, max_size=1_000_000):
        await asyncio.Future()


if __name__ == "__main__":
    asyncio.run(main())
