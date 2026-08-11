import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";

type Props = {
  sessionId: string;
  accessToken: string;
  className?: string;
  onError?: (msg: string) => void;
};

export function WebRTCViewer({ sessionId, accessToken, className, onError }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [status, setStatus] = useState("connecting");
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const sigRef = useRef<WebSocket | null>(null);
  const ctrlRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    let cancelled = false;
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const host = window.location.host;

    async function start() {
      try {
        const ice = await api.getIceServers(accessToken);
        const pc = new RTCPeerConnection({ iceServers: ice.iceServers });
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
          setStatus(pc.connectionState);
        };

        const sig = new WebSocket(
          `${proto}://${host}/ws/sessions/${sessionId}/signaling?token=${encodeURIComponent(accessToken)}`,
        );
        sigRef.current = sig;

        pc.onicecandidate = (ev) => {
          if (!ev.candidate || sig.readyState !== WebSocket.OPEN) return;
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

        sig.onmessage = async (msg) => {
          try {
            const data = JSON.parse(String(msg.data)) as {
              type: string;
              sdp?: string;
              candidate?: RTCIceCandidateInit;
            };
            if (data.type === "answer" && data.sdp) {
              await pc.setRemoteDescription({ type: "answer", sdp: data.sdp });
            } else if (data.type === "ice" && data.candidate) {
              await pc.addIceCandidate(data.candidate);
            } else if (data.type === "ready" || data.type === "peer-missing") {
              // renegotiate when agent joins
              if (pc.signalingState === "stable" || pc.signalingState === "have-local-offer") {
                /* wait */
              }
              if (!pc.localDescription) {
                const offer = await pc.createOffer();
                await pc.setLocalDescription(offer);
                sig.send(JSON.stringify({ type: "offer", sdp: offer.sdp }));
              }
            }
          } catch (err) {
            onError?.(err instanceof Error ? err.message : "webrtc signal error");
          }
        };

        sig.onopen = async () => {
          const offer = await pc.createOffer();
          await pc.setLocalDescription(offer);
          sig.send(JSON.stringify({ type: "offer", sdp: offer.sdp }));
        };

        const ctrl = new WebSocket(
          `${proto}://${host}/ws/sessions/${sessionId}/control?token=${encodeURIComponent(accessToken)}`,
        );
        ctrlRef.current = ctrl;

        if (cancelled) {
          pc.close();
          sig.close();
          ctrl.close();
        }
      } catch (err) {
        onError?.(err instanceof Error ? err.message : "webrtc failed");
        setStatus("failed");
      }
    }

    void start();
    return () => {
      cancelled = true;
      pcRef.current?.close();
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
