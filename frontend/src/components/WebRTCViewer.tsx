import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import { backoffMs } from "../lib/reconnect";

type Props = {
  sessionId: string;
  accessToken: string;
  className?: string;
  onError?: (msg: string) => void;
};

// No console output anywhere in this component previously -- when WebRTC gets stuck on
// "connecting" over a real network there is zero visibility into whether the effect
// restarted, the signaling socket dropped, or an offer was a fresh createOffer() vs a
// resend of the cached one. logId is a short per-effect-run tag so a log dump can tell
// those apart at a glance.
function log(logId: string, msg: string) {
  // eslint-disable-next-line no-console
  console.log(`[${(performance.now() / 1000).toFixed(3)}] [webrtc:${logId}] ${msg}`);
}

export function WebRTCViewer({ sessionId, accessToken, className, onError }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [status, setStatus] = useState("connecting");
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const sigRef = useRef<WebSocket | null>(null);
  const ctrlRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const logId = Math.random().toString(36).slice(2, 7);
    log(logId, "effect run (mount or a dependency changed)");
    let cancelled = false;
    let sigAttempt = 0;
    let ctrlAttempt = 0;
    let sigReconnectTimer: number | undefined;
    let ctrlReconnectTimer: number | undefined;
    let offerRetryTimer: number | undefined;
    let lastOfferSdp: string | null = null;
    let answered = false;
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const host = window.location.host;

    async function sendOffer(pc: RTCPeerConnection, sig: WebSocket) {
      if (!lastOfferSdp) {
        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        lastOfferSdp = offer.sdp ?? null;
        log(logId, `sendOffer: fresh createOffer (sdp len=${lastOfferSdp?.length ?? 0})`);
      } else {
        log(logId, `sendOffer: resending cached offer (sdp len=${lastOfferSdp.length})`);
      }
      if (lastOfferSdp && sig.readyState === WebSocket.OPEN) {
        sig.send(JSON.stringify({ type: "offer", sdp: lastOfferSdp }));
      } else {
        log(logId, `sendOffer: sig not open (readyState=${sig.readyState}), not sent`);
      }
    }

    // The signaling hub drops (not queues) a message when the agent side hasn't joined
    // the room yet and replies "peer-missing" -- without a retry the viewer's very first
    // offer can vanish and it just sits on "connecting" forever.
    function scheduleOfferRetry(pc: RTCPeerConnection, sig: WebSocket) {
      if (cancelled || answered) return;
      log(logId, "peer-missing received, scheduling offer retry in 1500ms");
      window.clearTimeout(offerRetryTimer);
      offerRetryTimer = window.setTimeout(() => {
        if (cancelled || answered) return;
        void sendOffer(pc, sig);
      }, 1500);
    }

    function connectSignaling(pc: RTCPeerConnection) {
      const sig = new WebSocket(
        `${proto}://${host}/ws/sessions/${sessionId}/signaling?token=${encodeURIComponent(accessToken)}`,
      );
      sigRef.current = sig;
      log(logId, `signaling: connecting (attempt ${sigAttempt})`);

      sig.onopen = () => {
        sigAttempt = 0;
        log(logId, "signaling: open");
        // Only (re)send the offer if we haven't connected yet -- a reconnect after the
        // handshake already succeeded shouldn't re-trigger renegotiation on the agent.
        if (!answered) void sendOffer(pc, sig);
      };

      sig.onmessage = async (msg) => {
        try {
          const data = JSON.parse(String(msg.data)) as {
            type: string;
            sdp?: string;
            candidate?: RTCIceCandidateInit;
          };
          if (data.type === "answer" && data.sdp) {
            log(logId, `signaling: answer received (sdp len=${data.sdp.length}), answered=true`);
            answered = true;
            window.clearTimeout(offerRetryTimer);
            await pc.setRemoteDescription({ type: "answer", sdp: data.sdp });
          } else if (data.type === "ice" && data.candidate) {
            await pc.addIceCandidate(data.candidate);
          } else if (data.type === "peer-missing") {
            scheduleOfferRetry(pc, sig);
          } else {
            log(logId, `signaling: message type=${data.type}`);
          }
        } catch (err) {
          onError?.(err instanceof Error ? err.message : "webrtc signal error");
        }
      };

      sig.onclose = (ev) => {
        log(logId, `signaling: closed (code=${ev.code}, reason=${ev.reason || "none"}, answered=${answered})`);
        if (cancelled) return;
        sigReconnectTimer = window.setTimeout(() => {
          if (cancelled) return;
          sigAttempt += 1;
          connectSignaling(pc);
        }, backoffMs(sigAttempt));
      };
      sig.onerror = () => sig.close();
    }

    function connectControl() {
      const ctrl = new WebSocket(
        `${proto}://${host}/ws/sessions/${sessionId}/control?token=${encodeURIComponent(accessToken)}`,
      );
      ctrlRef.current = ctrl;
      ctrl.onopen = () => {
        ctrlAttempt = 0;
      };
      ctrl.onclose = () => {
        if (cancelled) return;
        ctrlReconnectTimer = window.setTimeout(() => {
          if (cancelled) return;
          ctrlAttempt += 1;
          connectControl();
        }, backoffMs(ctrlAttempt));
      };
      ctrl.onerror = () => ctrl.close();
    }

    async function start() {
      try {
        const ice = await api.getIceServers(accessToken);
        if (cancelled) return;
        const pc = new RTCPeerConnection({ iceServers: ice.iceServers });
        log(logId, `pc created, iceServers=${JSON.stringify(ice.iceServers)}`);
        pcRef.current = pc;
        pc.addTransceiver("video", { direction: "recvonly" });
        pc.ontrack = (ev) => {
          if (videoRef.current && ev.streams[0]) {
            videoRef.current.srcObject = ev.streams[0];
            void videoRef.current.play().catch(() => undefined);
            setStatus("live");
          }
        };
        pc.onconnectionstatechange = () => {
          log(logId, `pc connectionState=${pc.connectionState} iceConnectionState=${pc.iceConnectionState}`);
          setStatus(pc.connectionState);
        };
        pc.onicecandidate = (ev) => {
          if (!ev.candidate) {
            log(logId, "local ICE gathering complete");
            return;
          }
          const sig = sigRef.current;
          if (!sig || sig.readyState !== WebSocket.OPEN) {
            log(logId, "local ICE candidate ready but signaling not open, dropped");
            return;
          }
          sig.send(
            JSON.stringify({
              type: "ice",
              candidate: {
                candidate: ev.candidate.candidate,
                sdpMid: ev.candidate.sdpMid,
                sdpMLineIndex: ev.candidate.sdpMLineIndex,
              },
            }),
          );
        };

        if (cancelled) {
          pc.close();
          return;
        }
        connectSignaling(pc);
        connectControl();
      } catch (err) {
        onError?.(err instanceof Error ? err.message : "webrtc failed");
        setStatus("failed");
      }
    }

    void start();
    return () => {
      log(logId, "effect cleanup (unmount or a dependency changed)");
      cancelled = true;
      window.clearTimeout(sigReconnectTimer);
      window.clearTimeout(ctrlReconnectTimer);
      window.clearTimeout(offerRetryTimer);
      pcRef.current?.close();
      pcRef.current = null;
      sigRef.current?.close();
      ctrlRef.current?.close();
    };
  }, [sessionId, accessToken, onError]);

  function sendControl(payload: Record<string, unknown>) {
    const ws = ctrlRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(payload));
    }
  }

  function mapCoords(e: React.MouseEvent<HTMLVideoElement>) {
    const el = e.currentTarget;
    const rect = el.getBoundingClientRect();
    const x = Math.round(((e.clientX - rect.left) / rect.width) * 1280);
    const y = Math.round(((e.clientY - rect.top) / rect.height) * 800);
    return { x, y };
  }

  return (
    <div className={className}>
      <video
        ref={videoRef}
        className="absolute inset-0 h-full w-full bg-black object-contain"
        autoPlay
        playsInline
        muted
        tabIndex={0}
        onMouseMove={(e) => {
          const { x, y } = mapCoords(e);
          sendControl({ type: "mousemove", x, y });
        }}
        onMouseDown={(e) => {
          e.preventDefault();
          const { x, y } = mapCoords(e);
          sendControl({ type: "mousemove", x, y });
          sendControl({ type: "mousedown", button: e.button === 2 ? 3 : 1 });
        }}
        onMouseUp={(e) => {
          sendControl({ type: "mouseup", button: e.button === 2 ? 3 : 1 });
        }}
        onContextMenu={(e) => e.preventDefault()}
        onWheel={(e) => sendControl({ type: "wheel", deltaY: e.deltaY })}
        onKeyDown={(e) => {
          e.preventDefault();
          sendControl({ type: "keydown", key: e.key.length === 1 ? e.key : e.code });
        }}
      />
      {status !== "live" && status !== "connected" && (
        <div className="pointer-events-none absolute inset-0 grid place-items-center bg-black/50 text-sm text-[var(--color-fog)]">
          WebRTC: {status}
        </div>
      )}
    </div>
  );
}
