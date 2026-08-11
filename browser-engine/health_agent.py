#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import socket
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


SESSION_ID = os.environ.get("SESSION_ID", "")


def port_open(port: int) -> bool:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.5):
            return True
    except OSError:
        return False


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:
        return

    def _json(self, code: int, payload: dict) -> None:
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path in ("/healthz", "/readyz"):
            ok = port_open(5900) and port_open(6080)
            self._json(
                200 if ok else 503,
                {
                    "status": "ok" if ok else "starting",
                    "sessionId": SESSION_ID,
                    "engine": "chromium",
                    "vnc": port_open(5900),
                    "novnc": port_open(6080),
                    "cdp": port_open(9222),
                },
            )
            return
        self._json(404, {"error": "not found"})


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8090), Handler).serve_forever()
