import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type HistoryDetail } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Badge, Button, Card, CardBody, CardHeader, Skeleton, StreamPlaceholder, ToolButton } from "../components/ui";
import {
  SessionNetworkPanel,
  normalizeHistoryNetworkEvents,
} from "../components/SessionNetworkPanel";

const REMOTE_W = 1280;
const REMOTE_H = 800;

type FrameItem = HistoryDetail["frames"][number];

export function HistoryDetailPage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const { accessToken, user } = useAuth();
  const nav = useNavigate();
  const [data, setData] = useState<HistoryDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [selectedIdx, setSelectedIdx] = useState(0);
  const [netOpen, setNetOpen] = useState(true);
  const [pcapBusy, setPcapBusy] = useState(false);
  const stripRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    if (!accessToken || !id) return;
    try {
      const d = await api.getHistory(accessToken, id);
      setData(d);
      setError(null);
      setSelectedIdx(0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken, id]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const frames = useMemo(() => {
    if (!data) return [] as FrameItem[];
    return [...data.frames]
      .filter((f) => f.hasImage)
      .sort((a, b) => a.createdAt.localeCompare(b.createdAt));
  }, [data]);

  const events = useMemo(
    () => normalizeHistoryNetworkEvents(data?.network || []),
    [data],
  );

  const selected = frames[selectedIdx] ?? frames[0] ?? null;

  useEffect(() => {
    if (selectedIdx >= frames.length && frames.length > 0) {
      setSelectedIdx(frames.length - 1);
    }
  }, [frames.length, selectedIdx]);

  useEffect(() => {
    const el = stripRef.current?.querySelector(`[data-frame-idx="${selectedIdx}"]`);
    el?.scrollIntoView({ behavior: "smooth", inline: "center", block: "nearest" });
  }, [selectedIdx]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        setSelectedIdx((i) => Math.max(0, i - 1));
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        setSelectedIdx((i) => Math.min(frames.length - 1, i + 1));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [frames.length]);

  async function onDelete() {
    if (!accessToken || !id) return;
    if (!window.confirm(t("history.deleteConfirm"))) return;
    setBusy(true);
    try {
      await api.deleteHistory(accessToken, id);
      nav("/history");
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  function selectFrame(idx: number) {
    if (idx < 0 || idx >= frames.length) return;
    setSelectedIdx(idx);
  }

  async function downloadPcap() {
    if (!accessToken || !id) return;
    setPcapBusy(true);
    try {
      const res = await fetch(api.pcapUrl(id), { headers: { Authorization: `Bearer ${accessToken}` } });
      if (!res.ok) throw new Error(await res.text());
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${id}.pcap`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "pcap download error");
    } finally {
      setPcapBusy(false);
    }
  }

  if (!data && !error) {
    return <Skeleton className="h-64" />;
  }

  const sess = data?.session;

  return (
    <div className="animate-fade-in space-y-4">
      <div className="ui-card flex flex-wrap items-center justify-between gap-3 px-5 py-4">
        <div>
          <Link to="/history" className="text-sm text-[var(--color-fog)] transition hover:text-[var(--color-snow)]">
            ← {t("history.back")}
          </Link>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-[var(--color-white)]">
            {sess?.name || t("history.title")}
            <span className="ml-3 font-mono text-sm text-[var(--color-muted)]">{id?.slice(0, 8)}</span>
          </h1>
          {sess && (
            <p className="mt-1 font-mono text-xs text-[var(--color-signal-2)]">
              {sess.status}
              {sess.browser ? ` · ${sess.browser}` : ""}
              {sess.stoppedAt ? ` · ${(sess.stoppedAt || "").replace("T", " ").slice(0, 19)}` : ""}
            </p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <ToolButton active={netOpen} onClick={() => setNetOpen((v) => !v)}>
            {t("viewer.network")}
          </ToolButton>
          {sess?.pcapAvailable && (
            <ToolButton onClick={() => void downloadPcap()} disabled={pcapBusy}>
              {pcapBusy ? t("viewer.pcapDownloading") : t("viewer.pcapDownload")}
            </ToolButton>
          )}
          {user?.role === "SUPER_ADMIN" && (
            <Button variant="danger" disabled={busy} onClick={() => void onDelete()}>
              {t("history.delete")}
            </Button>
          )}
        </div>
      </div>

      {error && <Alert tone="danger">{error}</Alert>}
      {sess?.netTaintChecked && (sess.netTaintTotal ?? 0) > 0 && (
        <Alert tone={(sess.netTaintFlagged ?? 0) > 0 ? "danger" : "success"}>
          {(sess.netTaintFlagged ?? 0) > 0
            ? t("viewer.netTaintFlagged", {
                flagged: sess.netTaintFlagged,
                total: sess.netTaintTotal,
                domains: (sess.netTaintDomains || []).join(", "),
              })
            : t("viewer.netTaintClean", { total: sess.netTaintTotal })}
        </Alert>
      )}

      <div
        className="relative w-full overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-line)] bg-black shadow-[var(--shadow-md)]"
        style={{ aspectRatio: `${REMOTE_W} / ${REMOTE_H}`, width: "100%" }}
      >
        {selected && id && accessToken ? (
          <AuthImage
            src={api.historyFrameUrl(id, selected.id)}
            token={accessToken}
            className="absolute inset-0 h-full w-full object-contain"
          />
        ) : (
          <StreamPlaceholder title={t("history.noFrames")} subtitle={t("history.timelineEmpty")} />
        )}
        {selected && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-4 pb-3 pt-10">
            <div className="flex flex-wrap items-end justify-between gap-2 text-xs">
              <div className="min-w-0">
                <Badge tone="neutral">
                  {selected.kind === "navigate" ? t("history.kindNavigate") : t("history.kindClick")}
                </Badge>
                {selected.url && (
                  <span className="ml-2 truncate font-mono text-[var(--color-fog)]">{selected.url}</span>
                )}
              </div>
              <span className="shrink-0 font-mono text-[var(--color-muted)]">
                {selected.createdAt.replace("T", " ").slice(0, 19)}
                {frames.length > 0 ? ` · ${selectedIdx + 1}/${frames.length}` : ""}
              </span>
            </div>
          </div>
        )}
      </div>

      {frames.length > 0 && (
        <Card>
          <CardHeader
            title={t("history.filmstrip")}
            action={
              <div className="flex gap-1">
                <ToolButton onClick={() => selectFrame(selectedIdx - 1)}>{t("history.prev")}</ToolButton>
                <ToolButton onClick={() => selectFrame(selectedIdx + 1)}>{t("history.next")}</ToolButton>
              </div>
            }
          />
          <CardBody>
            <div
              ref={stripRef}
              className="flex gap-2 overflow-x-auto pb-1 scroll-smooth"
              style={{ scrollbarWidth: "thin" }}
            >
              {frames.map((f, idx) => {
                const active = idx === selectedIdx;
                return (
                  <button
                    key={f.id}
                    type="button"
                    data-frame-idx={idx}
                    onClick={() => selectFrame(idx)}
                    className={[
                      "group relative h-20 w-32 shrink-0 overflow-hidden rounded-md border bg-black transition",
                      active
                        ? "border-[var(--color-signal)] ring-1 ring-[var(--color-signal)]/50"
                        : "border-[var(--color-line)] opacity-80 hover:opacity-100",
                    ].join(" ")}
                    title={f.createdAt.replace("T", " ").slice(0, 19)}
                  >
                    {id && accessToken && (
                      <AuthImage
                        src={api.historyFrameUrl(id, f.id)}
                        token={accessToken}
                        className="h-full w-full object-cover"
                        thumb
                      />
                    )}
                    <span className="absolute bottom-0 inset-x-0 bg-black/65 px-1 py-0.5 font-mono text-[9px] text-[var(--color-fog)]">
                      {String(idx + 1).padStart(2, "0")} ·{" "}
                      {f.kind === "navigate" ? t("history.kindNavigate") : t("history.kindClick")}
                    </span>
                  </button>
                );
              })}
            </div>
          </CardBody>
        </Card>
      )}

      {netOpen && <SessionNetworkPanel events={events} readOnly />}
    </div>
  );
}

function AuthImage({
  src,
  token,
  className,
  thumb,
}: {
  src: string;
  token: string;
  className?: string;
  thumb?: boolean;
}) {
  const [url, setUrl] = useState<string | null>(null);
  useEffect(() => {
    let revoked: string | null = null;
    let alive = true;
    void (async () => {
      const res = await fetch(src, { headers: { Authorization: `Bearer ${token}` } });
      const blob = await res.blob();
      if (!alive) return;
      revoked = URL.createObjectURL(blob);
      setUrl(revoked);
    })();
    return () => {
      alive = false;
      if (revoked) URL.revokeObjectURL(revoked);
    };
  }, [src, token]);
  if (!url) {
    return (
      <div
        className={
          thumb
            ? "h-full w-full animate-pulse bg-[var(--color-panel-3)]"
            : "absolute inset-0 animate-pulse bg-[var(--color-panel-3)]"
        }
      />
    );
  }
  return <img src={url} alt="" className={className} draggable={false} />;
}
