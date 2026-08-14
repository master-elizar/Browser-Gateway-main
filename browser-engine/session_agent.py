#!/usr/bin/env python3
"""Session agent: health, CDP/BiDi netmon push, clipboard, upload/download."""
from __future__ import annotations

import json
import mimetypes
import os
import queue
import socket
import subprocess
import threading
import time
import urllib.error
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse

try:
    import websocket  # type: ignore
except ImportError:  # pragma: no cover
    websocket = None

SESSION_ID = os.environ.get("SESSION_ID", "")
AGENT_TOKEN = os.environ.get("AGENT_TOKEN", "")
GATEWAY = os.environ.get("GATEWAY_INTERNAL_URL", "http://backend:8080").rstrip("/")
HOME = Path(os.environ.get("HOME", "/home/browser"))
DOWNLOADS = HOME / "Downloads"
UPLOADS = HOME / "Uploads"
DISPLAY = os.environ.get("DISPLAY", ":99")
BROWSER_ENGINE = (os.environ.get("BROWSER_ENGINE") or "chromium").strip().lower()

DOWNLOADS.mkdir(parents=True, exist_ok=True)
UPLOADS.mkdir(parents=True, exist_ok=True)

_seen_hosts: set[str] = set()
_host_ips: dict[str, set[str]] = {}
_lock = threading.Lock()

# CDP commands from HTTP handlers (clipboard paste) → consumed by netmon WS owner.
_cdp_cmds: queue.Queue = queue.Queue()
_cdp_pending: dict[int, dict] = {}
_cdp_pending_lock = threading.Lock()
_history_lock = threading.Lock()
_history_last_capture = 0.0
_history_last_url = ""
_history_count = 0
HISTORY_MAX = 500
HISTORY_DEBOUNCE = 0.35


def is_ip_literal(host: str) -> bool:
    if not host:
        return False
    try:
        socket.inet_pton(socket.AF_INET, host)
        return True
    except OSError:
        pass
    try:
        socket.inet_pton(socket.AF_INET6, host.strip("[]"))
        return True
    except OSError:
        return False


def resolve_host(host: str) -> list[str]:
    """Best-effort A/AAAA lookup for DNS/IP tabs (container resolver)."""
    if not host or is_ip_literal(host):
        return [host] if host and is_ip_literal(host) else []
    out: list[str] = []
    seen: set[str] = set()
    try:
        for info in socket.getaddrinfo(host, None):
            ip = info[4][0]
            if ip and ip not in seen:
                seen.add(ip)
                out.append(ip)
    except OSError:
        pass
    return out


def remember_host_ip(host: str, ip: str) -> bool:
    """Track host→IP; return True if this IP is newly seen for the host."""
    if not host or not ip:
        return False
    with _lock:
        bucket = _host_ips.setdefault(host, set())
        if ip in bucket:
            return False
        bucket.add(ip)
        return True


def port_open(port: int) -> bool:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.5):
            return True
    except OSError:
        return False


def gateway_post(path: str, payload: dict) -> None:
    if not AGENT_TOKEN or not SESSION_ID:
        return
    data = json.dumps(payload).encode()
    req = urllib.request.Request(
        f"{GATEWAY}{path}",
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
        print(f"[agent] gateway post {path}: {exc}", flush=True)


def cdp_best_target() -> tuple[str, str] | None:
    """(targetId, webSocketDebuggerUrl) for the current best CDP page target, or None.

    The id lets callers detect when Chromium has swapped the attached target out from
    under an already-open connection (e.g. a process swap on cross-origin navigation) --
    see run_cdp_netmon, which reconnects when this changes instead of silently listening
    to a target that no longer receives any events.
    """
    try:
        with urllib.request.urlopen("http://127.0.0.1:9222/json/list", timeout=2) as resp:
            targets = json.loads(resp.read().decode())
        for t in targets:
            if t.get("type") == "page" and t.get("webSocketDebuggerUrl"):
                return t.get("id") or "", t["webSocketDebuggerUrl"]
        for t in targets:
            if t.get("webSocketDebuggerUrl"):
                return t.get("id") or "", t["webSocketDebuggerUrl"]
    except Exception:
        pass
    try:
        with urllib.request.urlopen("http://127.0.0.1:9222/json/version", timeout=2) as resp:
            info = json.loads(resp.read().decode())
            ws_url = info.get("webSocketDebuggerUrl")
            if ws_url:
                return "", ws_url
    except Exception:
        pass
    return None


def cdp_ws_url() -> str | None:
    # Prefer a page target; browser-level WS often 403 without allow-origins.
    target = cdp_best_target()
    return target[1] if target else None


def push_events(events: list[dict]) -> None:
    if not events:
        return
    gateway_post(f"/internal/sessions/{SESSION_ID}/netmon", {"events": events})


def headers_to_dict(raw) -> dict:
    """Normalize CDP dict or BiDi [{name,value}] headers into a plain dict."""
    if not raw:
        return {}
    if isinstance(raw, dict):
        return {str(k): str(v) for k, v in raw.items()}
    out: dict[str, str] = {}
    if isinstance(raw, list):
        for item in raw:
            if not isinstance(item, dict):
                continue
            name = item.get("name")
            if not name:
                continue
            val = item.get("value")
            if isinstance(val, dict):
                val = val.get("value", "")
            out[str(name)] = str(val if val is not None else "")
    return out


def header_value(headers: dict, *names: str) -> str:
    if not headers:
        return ""
    lower = {str(k).lower(): str(v) for k, v in headers.items()}
    for name in names:
        v = lower.get(name.lower())
        if v:
            return v
    return ""


def initiator_meta(raw) -> dict | None:
    """Keep a compact initiator for request-tree linking."""
    if not isinstance(raw, dict):
        return None
    out: dict[str, str] = {}
    t = raw.get("type")
    if t:
        out["type"] = str(t)
    u = raw.get("url")
    if not u:
        stack = raw.get("stack") or {}
        frames = stack.get("callFrames") if isinstance(stack, dict) else None
        if isinstance(frames, list) and frames:
            u = (frames[0] or {}).get("url")
    if u:
        out["url"] = str(u)
    return out or None


def pending_from_cdp(params: dict, now: str) -> dict:
    req = params.get("request") or {}
    headers = headers_to_dict(req.get("headers"))
    init = initiator_meta(params.get("initiator"))
    document_url = (params.get("documentURL") or "").strip()
    if not document_url:
        document_url = header_value(headers, "Referer", "referer")
    resource_type = (params.get("type") or "").strip()
    pending: dict = {
        "method": req.get("method") or "GET",
        "url": req.get("url") or "",
        "requestHeaders": headers,
        "ts": now,
    }
    if document_url:
        pending["documentURL"] = document_url
    if init:
        pending["initiator"] = init
    if resource_type:
        pending["resourceType"] = resource_type
    return pending


def pending_from_bidi(params: dict, now: str) -> dict:
    req = params.get("request") or {}
    headers = headers_to_dict(req.get("headers"))
    document_url = header_value(headers, "Referer", "referer")
    # BiDi destination ≈ Fetch destination (document, script, style, …)
    dest = (req.get("destination") or params.get("destination") or "").strip()
    resource_type = ""
    if dest:
        resource_type = dest[:1].upper() + dest[1:] if dest else ""
        # Normalize common BiDi destinations toward CDP resourceType names.
        mapping = {
            "Document": "Document",
            "Script": "Script",
            "Style": "Stylesheet",
            "Stylesheet": "Stylesheet",
            "Image": "Image",
            "Font": "Font",
            "Xhr": "XHR",
            "Fetch": "Fetch",
            "Websocket": "WebSocket",
            "": "",
        }
        resource_type = mapping.get(resource_type, resource_type)
    init_raw = params.get("initiator")
    init = initiator_meta(init_raw) if isinstance(init_raw, dict) else None
    if init is None and params.get("initiatorType"):
        init = {"type": str(params.get("initiatorType"))}
    pending: dict = {
        "method": req.get("method") or "GET",
        "url": req.get("url") or "",
        "requestHeaders": headers,
        "ts": now,
    }
    if document_url:
        pending["documentURL"] = document_url
    if init:
        pending["initiator"] = init
    if resource_type:
        pending["resourceType"] = resource_type
    return pending


def build_http_event(base: dict, resp: dict, now: str) -> dict:
    url = base.get("url") or resp.get("url") or ""
    http_ev: dict = {
        "type": "http",
        "ts": base.get("ts") or now,
        "method": base.get("method") or "GET",
        "url": url,
        "status": resp.get("status"),
        "requestHeaders": base.get("requestHeaders") or {},
        "responseHeaders": headers_to_dict(resp.get("headers")),
    }
    remote_ip = (resp.get("remoteIPAddress") or "").strip()
    if remote_ip:
        http_ev["remoteIP"] = remote_ip
    for key in ("documentURL", "initiator", "resourceType"):
        if base.get(key):
            http_ev[key] = base[key]
    if not http_ev.get("documentURL"):
        ref = header_value(http_ev.get("requestHeaders") or {}, "Referer", "referer")
        if ref:
            http_ev["documentURL"] = ref
    return http_ev


def emit_dns_for_host(host: str, now: str, batch: list[dict], answers: list[str] | None = None) -> None:
    if not host:
        return
    with _lock:
        first = host not in _seen_hosts
        _seen_hosts.add(host)
    if not first:
        return
    resolved = answers if answers is not None else resolve_host(host)
    for ip in resolved:
        remember_host_ip(host, ip)
    batch.append(
        {
            "type": "dns",
            "ts": now,
            "query": host,
            "qtype": "AAAA" if ":" in host else "A",
            "answers": resolved,
        }
    )


def run_cdp_netmon() -> None:
    if websocket is None:
        print("[agent] websocket module missing; CDP netmon disabled", flush=True)
        return
    msg_id = 0
    pending: dict[str, dict] = {}

    def next_id() -> int:
        nonlocal msg_id
        msg_id += 1
        return msg_id

    def flush_cdp_cmds(ws) -> None:
        while True:
            try:
                item = _cdp_cmds.get_nowait()
            except queue.Empty:
                break
            mid = next_id()
            item["id"] = mid
            with _cdp_pending_lock:
                _cdp_pending[mid] = item
            try:
                ws.send(
                    json.dumps(
                        {
                            "id": mid,
                            "method": item["method"],
                            "params": item.get("params") or {},
                        }
                    )
                )
            except Exception as exc:  # noqa: BLE001
                item["error"] = str(exc)
                item["event"].set()
                with _cdp_pending_lock:
                    _cdp_pending.pop(mid, None)

    def resolve_cdp_reply(msg: dict) -> bool:
        mid = msg.get("id")
        if not isinstance(mid, int):
            return False
        with _cdp_pending_lock:
            item = _cdp_pending.pop(mid, None)
        if not item:
            return False
        if "error" in msg:
            item["error"] = msg.get("error")
        else:
            item["result"] = msg.get("result")
        item["event"].set()
        return True

    while True:
        target = cdp_best_target()
        if not target:
            time.sleep(2)
            continue
        target_id, url = target
        try:
            ws = websocket.create_connection(
                url,
                timeout=5,
                header=["Origin: http://127.0.0.1:9222"],
            )
            ws.settimeout(0.25)
            ws.send(json.dumps({"id": next_id(), "method": "Network.enable", "params": {}}))
            ws.send(json.dumps({"id": next_id(), "method": "Page.enable", "params": {}}))
            ws.send(json.dumps({"id": next_id(), "method": "Runtime.enable", "params": {}}))
            ws.send(json.dumps({"id": next_id(), "method": "Page.reload", "params": {"ignoreCache": True}}))
            print(f"[agent] CDP Network.enabled + reload (target={target_id})", flush=True)
            last_target_check = time.monotonic()
            while True:
                flush_cdp_cmds(ws)
                try:
                    raw = ws.recv()
                except websocket.WebSocketTimeoutException:
                    flush_cdp_cmds(ws)
                    # Chromium can swap the attached target out from under this connection
                    # on certain navigations (process swap on cross-origin nav, etc.)
                    # without ever erroring it -- recv() just keeps timing out forever with
                    # nothing new to read. That's why this previously looked like "capture
                    # works for the first page load, then silently stops for the rest of
                    # the session" with zero errors anywhere. Periodically confirm the
                    # target we're attached to is still the one Chromium considers current;
                    # if not, reconnect onto whatever is current now.
                    now = time.monotonic()
                    if target_id and now - last_target_check > 3:
                        last_target_check = now
                        current = cdp_best_target()
                        if current and current[0] != target_id:
                            print(
                                f"[agent] CDP target changed ({target_id} -> {current[0]}), reconnecting",
                                flush=True,
                            )
                            break
                    continue
                except Exception:
                    break
                try:
                    msg = json.loads(raw)
                except json.JSONDecodeError:
                    continue
                if resolve_cdp_reply(msg):
                    continue
                method = msg.get("method")
                params = msg.get("params") or {}
                now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                batch: list[dict] = []

                if method == "Page.frameNavigated":
                    frame = params.get("frame") or {}
                    if not frame.get("parentId"):
                        nav_url = str(frame.get("url") or "")
                        if nav_url and not nav_url.startswith("chrome://") and not nav_url.startswith("about:"):
                            push_history_frame("navigate", {"url": nav_url}, url=nav_url)

                if method == "Network.requestWillBeSent":
                    req = params.get("request") or {}
                    req_url = req.get("url") or ""
                    request_id = params.get("requestId") or ""
                    pending[request_id] = pending_from_cdp(params, now)
                    host = urlparse(req_url).hostname
                    if host:
                        emit_dns_for_host(host, now, batch)

                elif method == "Network.responseReceived":
                    request_id = params.get("requestId") or ""
                    resp = params.get("response") or {}
                    base = pending.pop(request_id, None) or {
                        "method": "GET",
                        "url": resp.get("url") or "",
                        "requestHeaders": {},
                        "ts": now,
                    }
                    http_ev = build_http_event(base, resp, now)
                    batch.append(http_ev)
                    url = http_ev.get("url") or ""
                    remote_ip = (http_ev.get("remoteIP") or "").strip()
                    host = urlparse(url).hostname or ""
                    if host and remote_ip and remember_host_ip(host, remote_ip):
                        batch.append(
                            {
                                "type": "dns",
                                "ts": now,
                                "query": host,
                                "qtype": "A",
                                "answers": [remote_ip],
                                "source": "cdp-remoteIP",
                            }
                        )

                if batch:
                    print(f"[agent] push {len(batch)} events", flush=True)
                    push_events(batch)
        except Exception as exc:  # noqa: BLE001
            print(f"[agent] CDP loop: {exc}", flush=True)
            # Fail any waiting paste/commands so HTTP handlers do not hang.
            with _cdp_pending_lock:
                waiting = list(_cdp_pending.values())
                _cdp_pending.clear()
            for item in waiting:
                item["error"] = str(exc)
                item["event"].set()
            time.sleep(2)


def run_bidi_netmon() -> None:
    """Firefox WebDriver BiDi network capture; falls back to CDP when available."""
    if websocket is None:
        print("[agent] websocket module missing; BiDi netmon disabled", flush=True)
        return

    msg_id = 0
    pending: dict[str, dict] = {}
    completed: set[str] = set()

    def next_id() -> int:
        nonlocal msg_id
        msg_id += 1
        return msg_id

    def send(ws, method: str, params: dict | None = None) -> int:
        mid = next_id()
        ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
        return mid

    while True:
        if not port_open(9222):
            time.sleep(2)
            continue
        try:
            ws = None
            mode = "bidi"
            try:
                ws = websocket.create_connection("ws://127.0.0.1:9222/session", timeout=5)
            except Exception as bidi_exc:
                print(f"[agent] BiDi /session connect failed: {bidi_exc}; trying CDP", flush=True)
                cdp = cdp_ws_url()
                if not cdp:
                    time.sleep(2)
                    continue
                ws = websocket.create_connection(
                    cdp,
                    timeout=5,
                    header=["Origin: http://127.0.0.1:9222"],
                )
                mode = "cdp"

            assert ws is not None
            ws.settimeout(1)
            pending.clear()
            completed.clear()

            if mode == "cdp":
                send(ws, "Network.enable", {})
                send(ws, "Page.enable", {})
                send(ws, "Page.reload", {"ignoreCache": True})
                print("[agent] Firefox CDP Network.enabled", flush=True)
            else:
                send(ws, "session.new", {"capabilities": {}})
                deadline = time.time() + 3
                while time.time() < deadline:
                    try:
                        raw = ws.recv()
                        msg = json.loads(raw)
                        if msg.get("id") and ("result" in msg or "error" in msg):
                            if msg.get("error"):
                                print(f"[agent] session.new error: {msg.get('error')}", flush=True)
                            break
                    except websocket.WebSocketTimeoutException:
                        continue
                    except Exception:
                        break
                send(
                    ws,
                    "session.subscribe",
                    {
                        "events": [
                            "network.beforeRequestSent",
                            "network.responseStarted",
                            "network.responseCompleted",
                        ]
                    },
                )
                print("[agent] BiDi network events subscribed", flush=True)

            while True:
                try:
                    raw = ws.recv()
                except websocket.WebSocketTimeoutException:
                    continue
                except Exception:
                    break
                try:
                    msg = json.loads(raw)
                except json.JSONDecodeError:
                    continue

                method = msg.get("method")
                params = msg.get("params") or {}
                now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                batch: list[dict] = []

                if mode == "cdp":
                    if method == "Network.requestWillBeSent":
                        req = params.get("request") or {}
                        req_url = req.get("url") or ""
                        request_id = params.get("requestId") or ""
                        pending[request_id] = pending_from_cdp(params, now)
                        host = urlparse(req_url).hostname
                        if host:
                            emit_dns_for_host(host, now, batch)
                    elif method == "Network.responseReceived":
                        request_id = params.get("requestId") or ""
                        resp = params.get("response") or {}
                        base = pending.pop(request_id, None) or {
                            "method": "GET",
                            "url": resp.get("url") or "",
                            "requestHeaders": {},
                            "ts": now,
                        }
                        http_ev = build_http_event(base, resp, now)
                        batch.append(http_ev)
                        url = http_ev.get("url") or ""
                        remote_ip = (http_ev.get("remoteIP") or "").strip()
                        host = urlparse(url).hostname or ""
                        if host and remote_ip and remember_host_ip(host, remote_ip):
                            batch.append(
                                {
                                    "type": "dns",
                                    "ts": now,
                                    "query": host,
                                    "qtype": "A",
                                    "answers": [remote_ip],
                                    "source": "cdp-remoteIP",
                                }
                            )
                else:
                    if method == "network.beforeRequestSent":
                        req = params.get("request") or {}
                        req_id = str(req.get("request") or "")
                        req_url = req.get("url") or ""
                        pending[req_id] = pending_from_bidi(params, now)
                        host = urlparse(req_url).hostname
                        if host:
                            emit_dns_for_host(host, now, batch)

                    elif method == "network.responseCompleted":
                        req = params.get("request") or {}
                        resp = params.get("response") or {}
                        req_id = str(req.get("request") or "")
                        if req_id and req_id in completed:
                            continue
                        if req_id:
                            completed.add(req_id)
                        base = pending.pop(req_id, None) or pending_from_bidi(params, now)
                        http_ev = build_http_event(base, resp, now)
                        batch.append(http_ev)
                        host = urlparse(http_ev.get("url") or "").hostname
                        if host:
                            emit_dns_for_host(host, now, batch)

                    elif method == "network.responseStarted":
                        # Keep pending enriched; HTTP emitted on responseCompleted.
                        req = params.get("request") or {}
                        req_id = str(req.get("request") or "")
                        if req_id and req_id not in pending:
                            pending[req_id] = pending_from_bidi(params, now)

                if batch:
                    print(f"[agent] push {len(batch)} events ({mode})", flush=True)
                    push_events(batch)
        except Exception as exc:  # noqa: BLE001
            print(f"[agent] BiDi/CDP loop: {exc}", flush=True)
            time.sleep(2)


def xclip_get() -> str:
    try:
        out = subprocess.check_output(
            ["xclip", "-selection", "clipboard", "-o"],
            env={**os.environ, "DISPLAY": DISPLAY},
            stderr=subprocess.DEVNULL,
            timeout=3,
        )
        return out.decode("utf-8", errors="replace")
    except Exception:
        return ""


def xclip_set(text: str) -> None:
    data = text.encode("utf-8", errors="replace")
    env = {**os.environ, "DISPLAY": DISPLAY}
    for selection in ("clipboard", "primary"):
        try:
            subprocess.run(
                ["xclip", "-selection", selection, "-in"],
                input=data,
                env=env,
                timeout=3,
                check=False,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        except Exception as exc:  # noqa: BLE001
            print(f"[agent] xclip {selection}: {exc}", flush=True)


def xdotool(*args: str) -> int:
    env = {**os.environ, "DISPLAY": DISPLAY}
    try:
        completed = subprocess.run(["xdotool", *args], env=env, check=False, timeout=5)
        return int(completed.returncode)
    except Exception as exc:  # noqa: BLE001
        print(f"[agent] xdotool: {exc}", flush=True)
        return 1


def focus_browser_window() -> bool:
    """Bring Chromium/Firefox window to front under Xvfb."""
    searches = (
        ("--class", "chromium"),
        ("--class", "Chromium"),
        ("--class", "google-chrome"),
        ("--class", "Google-chrome"),
        ("--class", "firefox"),
        ("--class", "Firefox"),
        ("--name", "Chromium"),
        ("--name", "Firefox"),
    )
    for flag, value in searches:
        if xdotool("search", "--onlyvisible", flag, value, "windowactivate", "--sync") == 0:
            return True
    return False


def cdp_call(method: str, params: dict | None = None, timeout: float = 3.0) -> tuple[dict | None, str | None]:
    """Run a CDP method on the netmon-owned page websocket."""
    if BROWSER_ENGINE == "firefox":
        return None, "firefox"
    ev = threading.Event()
    item: dict = {
        "method": method,
        "params": params or {},
        "event": ev,
        "result": None,
        "error": None,
    }
    _cdp_cmds.put(item)
    if not ev.wait(timeout):
        return None, "timeout"
    if item.get("error") is not None:
        err = item["error"]
        if isinstance(err, dict):
            return None, str(err.get("message") or err)
        return None, str(err)
    result = item.get("result")
    return (result if isinstance(result, dict) else {"value": result}), None


def cdp_insert_text(text: str) -> bool:
    """Insert text into the focused DOM element via CDP (Chromium)."""
    if not text:
        return False
    # 1) Native CDP insert (works for most focused inputs/contenteditable).
    result, err = cdp_call("Input.insertText", {"text": text}, timeout=2.5)
    if err is None and result is not None:
        return True

    # 2) Fallback: write into document.activeElement via JS (React-friendly value setter).
    expr = (
        "(function(text){"
        "const el=document.activeElement;"
        "if(!el||el===document.body||el===document.documentElement){"
        "return {ok:false,reason:'no-focus'};}"
        "try{if(document.execCommand&&document.execCommand('insertText',false,text))"
        "{return {ok:true,method:'execCommand'};}}catch(e){}"
        "if(el.isContentEditable){"
        "el.focus();"
        "const sel=window.getSelection();"
        "if(sel&&sel.rangeCount){const r=sel.getRangeAt(0);r.deleteContents();"
        "r.insertNode(document.createTextNode(text));r.collapse(false);"
        "return {ok:true,method:'contentEditable'};}}"
        "if('value' in el && typeof el.value==='string'){"
        "const start=el.selectionStart!=null?el.selectionStart:el.value.length;"
        "const end=el.selectionEnd!=null?el.selectionEnd:el.value.length;"
        "const next=el.value.slice(0,start)+text+el.value.slice(end);"
        "const proto=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value')"
        "||Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value');"
        "if(proto&&proto.set){proto.set.call(el,next);}else{el.value=next;}"
        "try{el.selectionStart=el.selectionEnd=start+text.length;}catch(e){}"
        "el.dispatchEvent(new InputEvent('input',{bubbles:true,data:text,inputType:'insertText'}));"
        "el.dispatchEvent(new Event('change',{bubbles:true}));"
        "return {ok:true,method:'value'};}"
        "return {ok:false,reason:'unsupported'};"
        "})(" + json.dumps(text) + ")"
    )
    result, err = cdp_call(
        "Runtime.evaluate",
        {"expression": expr, "returnByValue": True, "awaitPromise": False},
        timeout=2.5,
    )
    if err or not result:
        if err:
            print(f"[agent] cdp insert fallback: {err}", flush=True)
        return False
    # CDP shape: { result: { type, value: { ok, method } } }
    remote = result.get("result")
    value = remote.get("value") if isinstance(remote, dict) else None
    if isinstance(value, dict):
        ok = bool(value.get("ok"))
        if not ok:
            print(f"[agent] cdp insert js: {value}", flush=True)
        return ok
    return False


def paste_into_focused(text: str) -> dict:
    """
    Put text on the X11 clipboard and insert it into the focused remote field.
    Prefer CDP (shared netmon socket); fall back to window focus + type / Ctrl+V.
    """
    xclip_set(text)
    if not text:
        return {"ok": True, "pasted": False, "method": "clipboard"}

    if BROWSER_ENGINE != "firefox" and cdp_insert_text(text):
        print("[agent] paste method=cdp", flush=True)
        return {"ok": True, "pasted": True, "method": "cdp"}

    focus_browser_window()
    time.sleep(0.08)

    # Prefer typing: Ctrl+V often returns success even when nothing is focused.
    if len(text) <= 8000 and xdotool("type", "--clearmodifiers", "--delay", "1", "--", text) == 0:
        print("[agent] paste method=xdotool-type", flush=True)
        return {"ok": True, "pasted": True, "method": "xdotool-type"}

    if xdotool("key", "--clearmodifiers", "ctrl+v") == 0:
        print("[agent] paste method=xdotool-ctrlv", flush=True)
        return {"ok": True, "pasted": True, "method": "xdotool"}

    print("[agent] paste method=clipboard-only", flush=True)
    return {"ok": True, "pasted": False, "method": "clipboard"}


def capture_screenshot_jpeg() -> bytes | None:
    """Capture current page as JPEG via CDP (full HD quality ~75)."""
    result, err = cdp_call(
        "Page.captureScreenshot",
        {"format": "jpeg", "quality": 75, "fromSurface": True},
        timeout=4.0,
    )
    if err or not result:
        print(f"[agent] screenshot: {err}", flush=True)
        return None
    b64 = result.get("data")
    if not b64:
        return None
    import base64

    try:
        return base64.b64decode(b64)
    except Exception:
        return None


def current_page_url() -> str:
    result, err = cdp_call(
        "Runtime.evaluate",
        {"expression": "location.href", "returnByValue": True},
        timeout=2.0,
    )
    if err or not result:
        return ""
    remote = result.get("result")
    if isinstance(remote, dict):
        v = remote.get("value")
        return str(v) if v else ""
    return ""


def push_history_frame(kind: str, meta: dict | None = None, url: str = "") -> None:
    """Debounced upload of a screenshot frame to the gateway."""
    global _history_last_capture, _history_count, _history_last_url
    if not AGENT_TOKEN or not SESSION_ID:
        return
    with _history_lock:
        now = time.time()
        if now - _history_last_capture < HISTORY_DEBOUNCE and kind == "click":
            return
        if _history_count >= HISTORY_MAX:
            return
        if kind == "navigate" and url and url == _history_last_url:
            return
        _history_last_capture = now
        if url:
            _history_last_url = url

    def _worker() -> None:
        global _history_count
        time.sleep(0.12)  # let UI settle after click
        img = capture_screenshot_jpeg()
        if not img:
            return
        import base64

        page_url = url or current_page_url()
        payload = {
            "kind": kind,
            "url": page_url,
            "ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "imageBase64": base64.b64encode(img).decode("ascii"),
            "meta": meta or {},
        }
        try:
            gateway_post(f"/internal/sessions/{SESSION_ID}/history/frame", payload)
            with _history_lock:
                _history_count += 1
        except Exception as exc:  # noqa: BLE001
            print(f"[agent] history frame: {exc}", flush=True)

    threading.Thread(target=_worker, name="history-frame", daemon=True).start()


def list_downloads() -> list[dict]:
    items = []
    for p in sorted(DOWNLOADS.iterdir(), key=lambda x: x.stat().st_mtime, reverse=True):
        if not p.is_file():
            continue
        st = p.stat()
        items.append(
            {
                "id": p.name,
                "name": p.name,
                "size": st.st_size,
                "modifiedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(st.st_mtime)),
            }
        )
    return items


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:
        return

    def _json(self, code: int, payload: object) -> None:
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_json(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        if length <= 0:
            return {}
        return json.loads(self.rfile.read(length).decode() or "{}")

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path

        if path in ("/healthz", "/readyz"):
            ok = port_open(5900) and port_open(6080)
            self._json(
                200 if ok else 503,
                {
                    "status": "ok" if ok else "starting",
                    "sessionId": SESSION_ID,
                    "engine": BROWSER_ENGINE or "chromium",
                    "vnc": port_open(5900),
                    "novnc": port_open(6080),
                    "cdp": port_open(9222),
                    "agent": True,
                },
            )
            return

        if path == "/clipboard":
            self._json(200, {"text": xclip_get()})
            return

        if path == "/downloads":
            self._json(200, {"items": list_downloads()})
            return

        if path.startswith("/downloads/"):
            name = path[len("/downloads/") :]
            name = os.path.basename(name)
            target = DOWNLOADS / name
            if not target.is_file():
                self._json(404, {"error": "not found"})
                return
            data = target.read_bytes()
            ctype = mimetypes.guess_type(name)[0] or "application/octet-stream"
            self.send_response(200)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(data)))
            self.send_header("Content-Disposition", f'attachment; filename="{name}"')
            self.end_headers()
            self.wfile.write(data)
            return

        self._json(404, {"error": "not found"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/clipboard":
            body = self._read_json()
            text = str(body.get("text") or "")
            # Default: paste into the focused field (0.16.5+). Set paste=false for clipboard-only.
            paste = body.get("paste", True)
            if paste is False or str(paste).lower() in ("0", "false", "no"):
                xclip_set(text)
                self._json(200, {"ok": True, "pasted": False, "method": "clipboard"})
                return
            self._json(200, paste_into_focused(text))
            return

        if path == "/history/capture":
            body = self._read_json()
            kind = str(body.get("kind") or "click")
            meta = body.get("meta") if isinstance(body.get("meta"), dict) else {}
            push_history_frame(kind, meta)
            self._json(200, {"ok": True})
            return

        if path == "/upload":
            length = int(self.headers.get("Content-Length") or 0)
            ctype = self.headers.get("Content-Type") or ""
            raw = self.rfile.read(length) if length else b""
            filename = "upload.bin"
            file_bytes = raw
            if "multipart/form-data" in ctype and "boundary=" in ctype:
                boundary = ctype.split("boundary=")[-1].strip()
                parts = raw.split(f"--{boundary}".encode())
                for part in parts:
                    if b"Content-Disposition" not in part:
                        continue
                    header, _, content = part.partition(b"\r\n\r\n")
                    if b"filename=" in header:
                        for line in header.decode(errors="replace").split("\r\n"):
                            if "filename=" in line:
                                filename = line.split("filename=")[-1].strip().strip('"')
                                filename = os.path.basename(filename) or "upload.bin"
                        file_bytes = content.rstrip(b"\r\n--")
                        break
            else:
                qs = parse_qs(parsed.query)
                if "name" in qs:
                    filename = os.path.basename(qs["name"][0]) or filename
            dest = UPLOADS / filename
            dest.write_bytes(file_bytes)
            (DOWNLOADS / filename).write_bytes(file_bytes)
            self._json(200, {"ok": True, "name": filename, "size": len(file_bytes)})
            return

        self._json(404, {"error": "not found"})


def main() -> None:
    if BROWSER_ENGINE == "firefox":
        threading.Thread(target=run_bidi_netmon, name="bidi-netmon", daemon=True).start()
        print("[agent] netmon=BiDi/CDP (firefox)", flush=True)
    else:
        threading.Thread(target=run_cdp_netmon, name="cdp-netmon", daemon=True).start()
        print("[agent] netmon=CDP (chromium)", flush=True)
    print(f"[agent] listening :8090 session={SESSION_ID} engine={BROWSER_ENGINE}", flush=True)
    ThreadingHTTPServer(("0.0.0.0", 8090), Handler).serve_forever()


if __name__ == "__main__":
    main()
