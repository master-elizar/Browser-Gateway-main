#!/usr/bin/env python3
"""dnsmasq log tail → gateway netmon DNS events."""
from __future__ import annotations

import json
import os
import re
import threading
import time
import urllib.request

SESSION_ID = os.environ.get("SESSION_ID", "")
AGENT_TOKEN = os.environ.get("AGENT_TOKEN", "")
GATEWAY = os.environ.get("GATEWAY_INTERNAL_URL", "http://backend:8080").rstrip("/")
LOG_PATH = os.environ.get("DNSMASQ_LOG", "/home/browser/logs/dnsmasq.log")

QUERY_RE = re.compile(
    r"query\[(?P<qtype>\w+)\]\s+(?P<query>\S+)\s+from\s+\S+",
    re.I,
)
REPLY_RE = re.compile(
    r"(?:reply|is)\s+(?P<query>\S+)\s+is\s+(?P<answer>\S+)",
    re.I,
)


def push(events: list[dict]) -> None:
    if not events or not SESSION_ID or not AGENT_TOKEN:
        return
    data = json.dumps({"events": events}).encode()
    req = urllib.request.Request(
        f"{GATEWAY}/internal/sessions/{SESSION_ID}/netmon",
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {AGENT_TOKEN}",
            "X-Agent-Token": AGENT_TOKEN,
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            resp.read()
    except Exception as exc:  # noqa: BLE001
        print(f"[dns-tap] push: {exc}", flush=True)


def follow() -> None:
    pending: dict[str, dict] = {}
    while not os.path.exists(LOG_PATH):
        time.sleep(0.5)
    with open(LOG_PATH, "r", encoding="utf-8", errors="replace") as f:
        f.seek(0, 2)
        while True:
            line = f.readline()
            if not line:
                time.sleep(0.2)
                continue
            line = line.strip()
            now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
            mq = QUERY_RE.search(line)
            if mq:
                q = mq.group("query").rstrip(".")
                pending[q] = {
                    "type": "dns",
                    "ts": now,
                    "query": q,
                    "qtype": mq.group("qtype").upper(),
                    "answers": [],
                    "source": "dnsmasq",
                }
                continue
            mr = REPLY_RE.search(line)
            if mr:
                q = mr.group("query").rstrip(".")
                ans = mr.group("answer").rstrip(".")
                ev = pending.pop(q, None) or {
                    "type": "dns",
                    "ts": now,
                    "query": q,
                    "qtype": "A",
                    "answers": [],
                    "source": "dnsmasq",
                }
                if ans and ans not in ev["answers"] and ans.lower() != "nxdomain":
                    # skip CNAMEs that look like hostnames without digits? keep all
                    ev["answers"].append(ans)
                ev["ts"] = now
                push([ev])


if __name__ == "__main__":
    print("[dns-tap] starting", flush=True)
    follow()
