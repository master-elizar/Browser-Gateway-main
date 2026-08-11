import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  api,
  type BrowserSession,
  type DownloadItem,
  type NetworkEventItem,
  type TIResult,
  type ViewerFeatures,
} from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { WebRTCViewer } from "../components/WebRTCViewer";
import { SessionNetworkPanel, type NetTab } from "../components/SessionNetworkPanel";

type FitMode = "fit" | "stretch";
type StreamMode = "webrtc" | "novnc";

const REMOTE_W = 1280;
const REMOTE_H = 800;

const defaultViewerFeatures: ViewerFeatures = {
  viewerWebrtcEnabled: true,
  viewerNovncEnabled: true,
  viewerFitEnabled: true,
  viewerStretchEnabled: true,
  viewerClipboardEnabled: true,
  viewerUploadEnabled: true,
  viewerDownloadsEnabled: true,
  viewerNetworkEnabled: true,
};

function hostOf(urlOrHost?: string): string {
  if (!urlOrHost) return "";
  try {
    if (urlOrHost.includes("://")) return new URL(urlOrHost).hostname;
  } catch {
    /* ignore */
  }
  return urlOrHost.split("/")[0] || "";
}

function tiKey(value?: string, kind?: string): string {
  if (!value) return "";
  const v = value.trim().toLowerCase();
  if (kind === "url" || v.includes("://")) return v;
  const h = hostOf(v);
  return (h || v).replace(/^\[|\]$/g, "");
}

export function SessionViewerPage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [netOpen, setNetOpen] = useState(true);
  const [session, setSession] = useState<BrowserSession | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [events, setEvents] = useState<NetworkEventItem[]>([]);
  const [downloads, setDownloads] = useState<DownloadItem[]>([]);
  const [showDownloads, setShowDownloads] = useState(false);
  const [dlPick, setDlPick] = useState<DownloadItem | null>(null);
  const [dlFormat, setDlFormat] = useState<"file" | "zip">("file");
  const [dlPassword, setDlPassword] = useState("");
  const [dlBusy, setDlBusy] = useState(false);
  const [zipDefaultSet, setZipDefaultSet] = useState(false);
  const [fitMode, setFitMode] = useState<FitMode>("fit");
  const [streamMode, setStreamMode] = useState<StreamMode>("webrtc");
  const [features, setFeatures] = useState<ViewerFeatures>(defaultViewerFeatures);
  const [viewerHeight, setViewerHeight] = useState(640);
  const [clearBusy, setClearBusy] = useState(false);
  const [enrichBusy, setEnrichBusy] = useState(false);
  const [lookupBusy, setLookupBusy] = useState<string | null>(null);
  const [tiOverlay, setTiOverlay] = useState<Record<string, TIResult>>({});
  const fileRef = useRef<HTMLInputElement>(null);
  const frameWrapRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);
  const dragMoved = useRef(false);

  useEffect(() => {
    if (!accessToken) return;
    let alive = true;
    void api
      .getViewerFeatures(accessToken)
      .then((f) => {
        if (alive) setFeatures(f);
      })
      .catch(() => {
        /* keep defaults */
      });
    return () => {
      alive = false;
    };
  }, [accessToken]);

  useEffect(() => {
    if (streamMode === "webrtc" && !features.viewerWebrtcEnabled && features.viewerNovncEnabled) {
      setStreamMode("novnc");
    } else if (streamMode === "novnc" && !features.viewerNovncEnabled && features.viewerWebrtcEnabled) {
      setStreamMode("webrtc");
    }
  }, [features.viewerWebrtcEnabled, features.viewerNovncEnabled, streamMode]);

  useEffect(() => {
    if (fitMode === "fit" && !features.viewerFitEnabled && features.viewerStretchEnabled) {
      setFitMode("stretch");
    } else if (fitMode === "stretch" && !features.viewerStretchEnabled && features.viewerFitEnabled) {
      setFitMode("fit");
    }
  }, [features.viewerFitEnabled, features.viewerStretchEnabled, fitMode]);

  useEffect(() => {
    if (!features.viewerNetworkEnabled) setNetOpen(false);
  }, [features.viewerNetworkEnabled]);

  useEffect(() => {
    if (!features.viewerDownloadsEnabled) setShowDownloads(false);
  }, [features.viewerDownloadsEnabled]);

  useEffect(() => {
    if (!accessToken || !id) return;
    let alive = true;
    const tick = async () => {
      try {
        const s = await api.getSession(accessToken, id);
        if (alive) {
          setSession(s);
          setError(null);
        }
      } catch (err) {
        if (alive) setError(err instanceof Error ? err.message : "error");
      }
    };
    void tick();
    const timer = setInterval(() => void tick(), 2000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [accessToken, id]);

  useEffect(() => {
    if (!features.viewerNetworkEnabled || !accessToken || !id || session?.status !== "RUNNING") return;
    let closed = false;
    const load = async () => {
      try {
        const res = await api.listNetworkEvents(accessToken, id);
        if (!closed) setEvents(res.items.slice(-400));
      } catch {
        /* ignore */
      }
    };
    void load();

    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(
      `${proto}://${window.location.host}/ws/sessions/${id}/netmon?token=${encodeURIComponent(accessToken)}`,
    );
    ws.onmessage = (msg) => {
      try {
        const ev = JSON.parse(String(msg.data)) as NetworkEventItem;
        if (!ev.type || ev.type === "status") return;
        setEvents((prev) => [...prev.slice(-399), ev]);
      } catch {
        /* ignore */
      }
    };
    return () => {
      closed = true;
      ws.close();
    };
  }, [accessToken, id, session?.status, features.viewerNetworkEnabled]);

  // Fit mode uses CSS aspect-ratio — do not chase height in JS (that remounted the iframe).
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!dragging.current || fitMode !== "stretch") return;
      dragMoved.current = true;
      const el = frameWrapRef.current;
      if (!el) return;
      const top = el.getBoundingClientRect().top;
      const next = Math.min(Math.max(e.clientY - top, 320), 1200);
      setViewerHeight(next);
    };
    const onUp = () => {
      dragging.current = false;
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [fitMode]);

  const effectiveFit: FitMode =
    fitMode === "stretch" && !features.viewerStretchEnabled
      ? "fit"
      : fitMode === "fit" && !features.viewerFitEnabled && features.viewerStretchEnabled
        ? "stretch"
        : features.viewerFitEnabled || features.viewerStretchEnabled
          ? fitMode
          : "fit";
  const streamAvailable = features.viewerWebrtcEnabled || features.viewerNovncEnabled;
  const effectiveStream: StreamMode | null = !streamAvailable
    ? null
    : streamMode === "webrtc" && features.viewerWebrtcEnabled
      ? "webrtc"
      : features.viewerNovncEnabled
        ? "novnc"
        : features.viewerWebrtcEnabled
          ? "webrtc"
          : null;

  const resizeParam = effectiveFit === "stretch" ? "remote" : "scale";
  // Stable src: only changes when session/token/fitMode changes — never on height.
  const streamSrc =
    accessToken && id && session?.status === "RUNNING" && features.viewerNovncEnabled
      ? `/novnc/embed.html?autoconnect=1&reconnect=1&reconnect_delay=2000&resize=${resizeParam}&path=${encodeURIComponent(
          `ws/sessions/${id}/vnc?token=${accessToken}`,
        )}`
      : null;


  async function stop() {
    if (!accessToken || !id) return;
    try {
      await api.stopSession(accessToken, id);
      const s = await api.getSession(accessToken, id);
      setSession(s);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function copyToRemote() {
    if (!accessToken || !id) return;
    try {
      const text = await navigator.clipboard.readText();
      await api.clipboard(accessToken, id, "toRemote", text);
      setNote(t("viewer.clipboardSent"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "clipboard error");
    }
  }

  async function copyFromRemote() {
    if (!accessToken || !id) return;
    try {
      const res = await api.clipboard(accessToken, id, "fromRemote");
      if (res.text != null) {
        await navigator.clipboard.writeText(res.text);
        setNote(t("viewer.clipboardReceived"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "clipboard error");
    }
  }

  async function onUpload(file: File | null) {
    if (!accessToken || !id || !file) return;
    try {
      const res = await api.upload(accessToken, id, file);
      setNote(`${t("viewer.uploaded")}: ${res.name || file.name}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "upload error");
    }
  }

  async function openDownloadPicker(item: DownloadItem) {
    setDlPick(item);
    setDlFormat("file");
    setDlPassword("");
    setZipDefaultSet(Boolean(features.downloadZipPasswordDefaultSet));
    if (!accessToken) return;
    try {
      const f = await api.getViewerFeatures(accessToken);
      setFeatures(f);
      setZipDefaultSet(Boolean(f.downloadZipPasswordDefaultSet));
    } catch {
      /* keep current */
    }
  }

  async function confirmDownload() {
    if (!accessToken || !id || !dlPick) return;
    setDlBusy(true);
    try {
      const opts =
        dlFormat === "zip"
          ? { format: "zip" as const, password: dlPassword || undefined }
          : { format: "file" as const };
      const res = await fetch(api.downloadUrl(id, dlPick.id, opts), {
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      if (!res.ok) throw new Error(await res.text());
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = dlFormat === "zip" ? `${dlPick.name}.zip` : dlPick.name;
      a.click();
      URL.revokeObjectURL(url);
      setDlPick(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "download error");
    } finally {
      setDlBusy(false);
    }
  }

  async function openDownloads() {
    if (!accessToken || !id) return;
    setShowDownloads(true);
    try {
      const res = await api.listDownloads(accessToken, id);
      setDownloads(res.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "downloads error");
    }
  }

  async function clearNetwork(scope: "tab" | "all", tab: NetTab = "all") {
    if (!accessToken || !id) return;
    setClearBusy(true);
    try {
      if (scope === "all" || tab === "all" || tab === "netflow" || tab === "url" || tab === "fqdn" || tab === "ip" || tab === "tree") {
        await api.clearNetworkEvents(accessToken, id, "all");
        setEvents([]);
        setTiOverlay({});
        setNote(t("viewer.clearedAll"));
      } else if (tab === "dns") {
        await api.clearNetworkEvents(accessToken, id, "dns");
        setEvents((prev) => prev.filter((e) => e.type !== "dns"));
        setNote(t("viewer.clearedTab"));
      } else if (tab === "http") {
        await api.clearNetworkEvents(accessToken, id, "http");
        setEvents((prev) => prev.filter((e) => e.type !== "http"));
        setNote(t("viewer.clearedTab"));
      } else if (tab === "ti") {
        setEvents((prev) => prev.filter((e) => e.type !== "ti"));
        setTiOverlay({});
        setNote(t("viewer.clearedTab"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "clear error");
    } finally {
      setClearBusy(false);
    }
  }

  async function enrichNetwork() {
    if (!accessToken || !id) return;
    setEnrichBusy(true);
    try {
      const res = await api.enrichNetwork(accessToken, id);
      setTiOverlay((prev) => {
        const next = { ...prev };
        for (const item of res.items || []) {
          if (!item?.indicator || item.error) continue;
          const k = tiKey(item.indicator);
          if (k) next[k] = item;
        }
        return next;
      });
      const tiEvents = (res.items || [])
        .filter((item) => item?.indicator && !item.error)
        .map((item) => ({
          type: "ti",
          ts: new Date().toISOString(),
          provider: item.provider,
          kind: item.kind,
          indicator: item.indicator,
          verdict: item.verdict,
          malicious: item.malicious,
          suspicious: item.suspicious,
          harmless: item.harmless,
          undetected: item.undetected,
          permalink: item.permalink,
          cached: item.cached,
          providers: item.providers,
        }));
      if (tiEvents.length) {
        setEvents((prev) => [...prev, ...tiEvents].slice(-400));
      }
      setNote(t("viewer.tiEnriched", { count: res.total ?? res.items?.length ?? 0 }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "TI enrich error");
    } finally {
      setEnrichBusy(false);
    }
  }

  async function lookupIndicator(value: string, kind?: string) {
    if (!accessToken || !value) return;
    const key = tiKey(value, kind);
    setLookupBusy(key);
    try {
      const res = await api.tiLookup(accessToken, { value, kind, sessionId: id });
      setTiOverlay((prev) => {
        const next = { ...prev };
        // Index under the requested key and the returned indicator so rows match.
        next[key] = res;
        if (res.indicator) {
          next[tiKey(res.indicator, res.kind)] = res;
          const host = hostOf(res.indicator);
          if (host) next[tiKey(host)] = res;
        }
        return next;
      });
      // Mirror into the live event list so the TI tab updates immediately;
      // backend also persists for History.
      if (res.indicator && !res.error) {
        setEvents((prev) => [
          ...prev.slice(-399),
          {
            type: "ti",
            ts: new Date().toISOString(),
            provider: res.provider,
            kind: res.kind,
            indicator: res.indicator,
            verdict: res.verdict,
            malicious: res.malicious,
            suspicious: res.suspicious,
            harmless: res.harmless,
            undetected: res.undetected,
            permalink: res.permalink,
            cached: res.cached,
            providers: res.providers,
          },
        ]);
      }
      if (res.error) setError(res.error);
    } catch (err) {
      setError(err instanceof Error ? err.message : "TI lookup error");
    } finally {
      setLookupBusy(null);
    }
  }

  return (
    <div className="animate-fade-in space-y-4">
      <div className="ui-card flex flex-wrap items-center justify-between gap-3 px-5 py-4">
        <div>
          <Link to="/sessions" className="text-sm text-[var(--color-fog)] transition hover:text-[var(--color-snow)]">
            ← {t("viewer.back")}
          </Link>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-[var(--color-white)]">
            {t("viewer.title")}
            <span className="ml-3 font-mono text-sm text-[var(--color-muted)]">{id?.slice(0, 8)}</span>
          </h1>
          {session && (
            <p className="mt-1 font-mono text-xs text-[var(--color-signal-2)]">
              {session.status}
              {session.containerId ? ` · ${session.containerId.slice(0, 12)}` : ""}
              {streamAvailable ? ` · ${effectiveFit}` : ""}
            </p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {features.viewerWebrtcEnabled && (
            <ToolBtn active={effectiveStream === "webrtc"} onClick={() => setStreamMode("webrtc")}>
              WebRTC
            </ToolBtn>
          )}
          {features.viewerNovncEnabled && (
            <ToolBtn active={effectiveStream === "novnc"} onClick={() => setStreamMode("novnc")}>
              noVNC
            </ToolBtn>
          )}
          {features.viewerFitEnabled && (
            <ToolBtn active={effectiveFit === "fit"} onClick={() => setFitMode("fit")}>
              {t("viewer.fit")}
            </ToolBtn>
          )}
          {features.viewerStretchEnabled && (
            <ToolBtn active={effectiveFit === "stretch"} onClick={() => setFitMode("stretch")}>
              {t("viewer.stretch")}
            </ToolBtn>
          )}
          {features.viewerClipboardEnabled && (
            <>
              <ToolBtn onClick={() => void copyToRemote()}>{t("viewer.clipboardTo")}</ToolBtn>
              <ToolBtn onClick={() => void copyFromRemote()}>{t("viewer.clipboardFrom")}</ToolBtn>
            </>
          )}
          {features.viewerUploadEnabled && (
            <ToolBtn onClick={() => fileRef.current?.click()}>{t("viewer.upload")}</ToolBtn>
          )}
          {features.viewerDownloadsEnabled && (
            <ToolBtn active={showDownloads} onClick={() => void openDownloads()}>
              {t("viewer.downloads")}
            </ToolBtn>
          )}
          {features.viewerNetworkEnabled && (
            <ToolBtn active={netOpen} onClick={() => setNetOpen((v) => !v)}>
              {t("viewer.network")}
            </ToolBtn>
          )}
          <button
            type="button"
            onClick={() => void stop()}
            className="rounded-lg border border-[var(--color-danger)]/50 px-3 py-1.5 text-sm text-[var(--color-danger)] hover:bg-[var(--color-danger)]/10"
          >
            {t("viewer.stop")}
          </button>
          {features.viewerUploadEnabled && (
            <input
              ref={fileRef}
              type="file"
              className="hidden"
              onChange={(e) => void onUpload(e.target.files?.[0] ?? null)}
            />
          )}
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-4 py-3 text-sm text-[var(--color-danger)]">
          {error}
        </div>
      )}
      {note && (
        <div className="rounded-lg border border-[var(--color-signal)]/40 bg-[var(--color-signal)]/10 px-4 py-3 text-sm text-[var(--color-signal-2)]">
          {note}
        </div>
      )}

      {features.viewerDownloadsEnabled && showDownloads && (
        <div className="ui-card p-4">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="section-title">{t("viewer.downloads")}</h2>
            <button type="button" className="text-xs text-[var(--color-fog)]" onClick={() => setShowDownloads(false)}>
              {t("common.close")}
            </button>
          </div>
          {downloads.length === 0 ? (
            <p className="text-sm text-[var(--color-fog)]">{t("viewer.downloadsEmpty")}</p>
          ) : (
            <ul className="space-y-2 text-sm">
              {downloads.map((d) => (
                <li key={d.id} className="flex items-center justify-between gap-3">
                  <span className="truncate font-mono text-xs">{d.name}</span>
                  <button
                    type="button"
                    className="shrink-0 text-[var(--color-signal-2)] hover:underline"
                    onClick={() => void openDownloadPicker(d)}
                  >
                    {t("viewer.download")}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {dlPick && (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4"
          onClick={() => !dlBusy && setDlPick(null)}
        >
          <div
            className="ui-card w-full max-w-md space-y-4 p-5"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="section-title">{t("viewer.downloadFormat")}</h2>
            <p className="truncate font-mono text-xs text-[var(--color-fog)]">{dlPick.name}</p>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="dlfmt"
                checked={dlFormat === "file"}
                onChange={() => setDlFormat("file")}
              />
              {t("viewer.downloadPlain")}
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="dlfmt"
                checked={dlFormat === "zip"}
                onChange={() => setDlFormat("zip")}
              />
              {t("viewer.downloadZip")}
            </label>
            {dlFormat === "zip" && (
              <div>
                <label className="mb-1 block text-xs text-[var(--color-fog)]">
                  {t("viewer.downloadZipPassword")}
                </label>
                <input
                  type="password"
                  className="ui-input w-full"
                  value={dlPassword}
                  placeholder={zipDefaultSet ? "••••••••" : ""}
                  onChange={(e) => setDlPassword(e.target.value)}
                  autoComplete="off"
                />
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                type="button"
                className="rounded border border-[var(--color-line)] px-3 py-1.5 text-sm"
                disabled={dlBusy}
                onClick={() => setDlPick(null)}
              >
                {t("viewer.downloadCancel")}
              </button>
              <button
                type="button"
                className="rounded bg-[var(--color-signal)] px-3 py-1.5 text-sm text-black disabled:opacity-50"
                disabled={dlBusy || (dlFormat === "zip" && !dlPassword && !zipDefaultSet)}
                onClick={() => void confirmDownload()}
              >
                {t("viewer.downloadConfirm")}
              </button>
            </div>
          </div>
        </div>
      )}

      <div
        ref={frameWrapRef}
        className="relative w-full overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-line)] bg-black shadow-[var(--shadow-md)]"
        style={
          effectiveFit === "fit"
            ? { aspectRatio: `${REMOTE_W} / ${REMOTE_H}`, width: "100%" }
            : { height: viewerHeight, width: "100%" }
        }
      >
        {session?.status === "RUNNING" || session?.status === "IDLE" ? (
          !streamAvailable || !effectiveStream ? (
            <div className="grid h-full place-items-center p-6 text-center">
              <div>
                <div className="text-lg text-[var(--color-snow)]">{t("viewer.streamDisabled")}</div>
                <p className="mt-2 text-sm text-[var(--color-fog)]">{t("viewer.streamDisabledHint")}</p>
              </div>
            </div>
          ) : effectiveStream === "webrtc" && accessToken && id ? (
            <WebRTCViewer
              sessionId={id}
              accessToken={accessToken}
              className="absolute inset-0"
              onError={(msg) => setError(msg)}
            />
          ) : streamSrc ? (
            <iframe
              key={`${id}-${effectiveFit}`}
              title="remote-browser"
              src={streamSrc}
              className="absolute inset-0 h-full w-full border-0 bg-black"
              allow="clipboard-read; clipboard-write"
            />
          ) : (
            <div className="grid h-full place-items-center p-6 text-center">
              <div>
                <div className="text-lg text-[var(--color-snow)]">{t("viewer.waitingContainer")}</div>
                <p className="mt-2 font-mono text-xs text-[var(--color-fog)]">{session?.status || "…"}</p>
              </div>
            </div>
          )
        ) : (
          <div className="grid h-full place-items-center p-6 text-center">
            <div>
              <div className="text-lg text-[var(--color-snow)]">{t("viewer.waitingContainer")}</div>
              <p className="mt-2 font-mono text-xs text-[var(--color-fog)]">{session?.status || "…"}</p>
            </div>
          </div>
        )}
        {effectiveFit === "stretch" && features.viewerStretchEnabled && (
          <div
            role="separator"
            aria-orientation="horizontal"
            title={t("viewer.dragResize")}
            onMouseDown={() => {
              dragging.current = true;
              dragMoved.current = false;
            }}
            className="absolute inset-x-0 bottom-0 z-10 flex h-3 cursor-ns-resize items-center justify-center bg-gradient-to-t from-black/70 to-transparent"
          >
            <div className="h-1 w-16 rounded-full bg-[var(--color-fog)]/70" />
          </div>
        )}
      </div>

      {effectiveFit === "stretch" && features.viewerStretchEnabled && (
        <div className="flex items-center gap-3 text-xs text-[var(--color-fog)]">
          <span>{t("viewer.height")}</span>
          <input
            type="range"
            min={320}
            max={1200}
            value={viewerHeight}
            onChange={(e) => setViewerHeight(Number(e.target.value))}
            className="w-full accent-[var(--color-signal)]"
          />
          <span className="w-12 font-mono">{viewerHeight}px</span>
        </div>
      )}

      {features.viewerNetworkEnabled && netOpen && (
        <SessionNetworkPanel
          events={events}
          tiOverlay={tiOverlay}
          enrichBusy={enrichBusy}
          clearBusy={clearBusy}
          lookupBusy={lookupBusy}
          onEnrich={() => void enrichNetwork()}
          onClearTab={(tab) => void clearNetwork("tab", tab)}
          onClearAll={() => void clearNetwork("all")}
          onLookup={(value, kind) => void lookupIndicator(value, kind)}
        />
      )}

    </div>
  );
}

function ToolBtn({
  children,
  active,
  onClick,
}: {
  children: ReactNode;
  active?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={[
        "rounded-[var(--radius-md)] border px-3 py-1.5 text-sm transition",
        active
          ? "border-[var(--color-signal)]/60 bg-[var(--color-signal-dim)] text-[var(--color-signal-2)] shadow-[var(--shadow-sm)]"
          : "border-[var(--color-line)] bg-[var(--color-panel)]/40 text-[var(--color-fog)] hover:border-[var(--color-line-strong)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-snow)]",
      ].join(" ")}
    >
      {children}
    </button>
  );
}
