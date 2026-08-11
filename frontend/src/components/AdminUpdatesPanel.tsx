import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";

type UpdateProgress = {
  percent: number;
  phase: string;
  message: string;
  updatedAt?: string;
  done: boolean;
  error?: string;
};

type UpdateInfo = {
  currentVersion: string;
  latestTag?: string;
  latestCommit?: string;
  installedCommit?: string;
  updateAvailable: boolean;
  checkedAt: string;
  htmlUrl?: string;
  updatePending: boolean;
  pendingStale?: boolean;
  error?: string;
  source?: string;
  checkFailed?: boolean;
  canForce?: boolean;
  progress?: UpdateProgress;
};

const PHASE_ORDER = ["queued", "fetch", "checkout", "prepare", "build", "engines", "restart", "complete"] as const;

function phaseIndex(phase: string): number {
  if (phase === "failed") return -1;
  const i = PHASE_ORDER.indexOf(phase as (typeof PHASE_ORDER)[number]);
  return i >= 0 ? i : 0;
}

export function AdminUpdatesPanel() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [info, setInfo] = useState<UpdateInfo | null>(null);
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [smoothPct, setSmoothPct] = useState(0);

  const refresh = useCallback(async () => {
    if (!accessToken) {
      throw new Error("not signed in");
    }
    const s = await api.checkUpdates(accessToken);
    setInfo(s);
    if (s.error) setError(s.error);
    else setError(null);
    return s;
  }, [accessToken]);

  const inFlight =
    !!info?.updatePending || (!!info?.progress && !info.progress.done);

  useEffect(() => {
    setChecking(true);
    void refresh()
      .catch((err) => setError(err instanceof Error ? err.message : "check failed"))
      .finally(() => setChecking(false));
  }, [refresh]);

  // Poll often while an update is running so the bar stays live.
  useEffect(() => {
    if (!accessToken) return;
    const ms = inFlight ? 2000 : 5 * 60 * 1000;
    const id = window.setInterval(() => {
      void refresh().catch(() => undefined);
    }, ms);
    return () => window.clearInterval(id);
  }, [accessToken, inFlight, refresh]);

  // Ease percent toward the server value.
  useEffect(() => {
    const target = Math.max(
      0,
      Math.min(100, info?.progress?.percent ?? (info?.updatePending ? 2 : 0)),
    );
    if (!info?.progress && !info?.updatePending) {
      setSmoothPct(0);
      return;
    }
    const id = window.setInterval(() => {
      setSmoothPct((prev) => {
        const next = prev + (target - prev) * 0.28;
        if (Math.abs(next - target) < 0.35) return target;
        return next;
      });
    }, 40);
    return () => window.clearInterval(id);
  }, [info?.progress?.percent, info?.progress, info?.updatePending]);

  async function onCheck() {
    setChecking(true);
    setMsg(null);
    setError(null);
    try {
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("admin.updatesCheckFailed"));
    } finally {
      setChecking(false);
    }
  }

  const progress = info?.progress;
  const showProgress = !!(progress || info?.updatePending);
  const pct = Math.round(smoothPct);
  const currentPhase = progress?.phase || (info?.updatePending ? "queued" : "");
  const failed = currentPhase === "failed" || !!progress?.error;
  const done = !!progress?.done && !failed;

  const canApply =
    !!accessToken &&
    !busy &&
    !!info &&
    (info.updateAvailable || !!info.pendingStale || (progress?.done && failed)) &&
    !info.updatePending;

  const canForce =
    !!accessToken &&
    !busy &&
    !!info &&
    !info.updatePending &&
    !canApply &&
    (info.canForce || info.checkFailed || !!info.error);
  const activeIdx = phaseIndex(currentPhase);

  const stepStates = useMemo(() => {
    return PHASE_ORDER.map((phase, idx) => {
      let state: "done" | "active" | "pending" | "failed" = "pending";
      if (failed && phase === "complete") state = "failed";
      else if (done || (activeIdx >= 0 && idx < activeIdx)) state = "done";
      else if (idx === activeIdx && !done) state = failed ? "failed" : "active";
      else if (failed && idx > activeIdx) state = "pending";
      return { phase, state };
    });
  }, [activeIdx, done, failed]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-[var(--color-fog)]">{t("admin.updatesHint")}</p>
        <button
          type="button"
          className="btn-ghost shrink-0"
          disabled={checking || !accessToken}
          onClick={() => void onCheck()}
        >
          {checking ? t("common.loading") : t("admin.updatesCheck")}
        </button>
      </div>

      {info && (
        <div className="rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-3 py-2 text-sm">
          <div>
            {t("admin.updatesCurrent")}: <span className="font-mono">{info.currentVersion}</span>
            {info.installedCommit && (
              <span className="ml-2 font-mono text-xs text-[var(--color-fog)]">
                @{info.installedCommit}
              </span>
            )}
          </div>
          <div>
            {t("admin.updatesLatest")}:{" "}
            <span className="font-mono">
              {info.latestTag || info.latestCommit || (info.checkFailed ? t("admin.updatesUnknownLatest") : "—")}
            </span>
            {info.latestCommit &&
              info.latestTag &&
              !String(info.latestTag).includes(info.latestCommit) && (
                <span className="ml-2 font-mono text-xs text-[var(--color-fog)]">
                  @{info.latestCommit}
                </span>
              )}
            {info.updateAvailable ? (
              <span className="ml-2 text-[var(--color-signal-2)]">{t("admin.updatesAvailable")}</span>
            ) : info.checkFailed ? (
              <span className="ml-2 text-[var(--color-warn)]">{t("admin.updatesCheckFailedShort")}</span>
            ) : (
              <span className="ml-2 text-[var(--color-fog)]">{t("admin.updatesNone")}</span>
            )}
          </div>
          {!info.installedCommit && (
            <div className="mt-1 text-xs text-[var(--color-fog)]">{t("admin.updatesUnknownCommit")}</div>
          )}
          <div className="text-xs text-[var(--color-fog)]">{info.checkedAt}</div>
          {info.htmlUrl && (
            <a
              className="text-xs text-[var(--color-signal-2)] underline"
              href={info.htmlUrl}
              target="_blank"
              rel="noreferrer"
            >
              {t("admin.updatesViewOnGitHub")}
            </a>
          )}
          {info.updatePending && !showProgress && (
            <div className="mt-1 text-[var(--color-warn)]">{t("admin.updatesPending")}</div>
          )}
          {info.pendingStale && (
            <div className="mt-1 text-[var(--color-warn)]">{t("admin.updatesPendingStale")}</div>
          )}
        </div>
      )}

      {showProgress && (
        <div
          className={`overflow-hidden rounded-[var(--radius-lg)] border px-4 py-4 ${
            failed
              ? "border-[var(--color-danger)]/35 bg-[var(--color-danger-dim)]"
              : done
                ? "border-[var(--color-success)]/30 bg-[var(--color-success-dim)]"
                : "border-[var(--color-line)] bg-[var(--color-panel)]"
          }`}
        >
          <div className="mb-3 flex items-end justify-between gap-3">
            <div className="min-w-0">
              <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--color-muted)]">
                {t("admin.updatesProgressTitle")}
              </div>
              <div className="mt-1 truncate text-sm font-medium text-[var(--color-snow)]">
                {progress?.message ||
                  t(`admin.updatesPhase.${currentPhase || "queued"}`, {
                    defaultValue: t("admin.updatesPending"),
                  })}
              </div>
            </div>
            <div
              className={`shrink-0 font-mono text-2xl font-semibold tabular-nums leading-none ${
                failed
                  ? "text-[var(--color-danger)]"
                  : done
                    ? "text-[var(--color-success)]"
                    : "text-[var(--color-signal-2)]"
              }`}
            >
              {failed ? "!" : `${pct}%`}
            </div>
          </div>

          {/* Progress toolbar 0–100 */}
          <div
            className="relative h-3 overflow-hidden rounded-full bg-[var(--color-panel-3)]"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={pct}
          >
            <div
              className={`absolute inset-y-0 left-0 rounded-full transition-[width] duration-500 ease-out ${
                failed
                  ? "bg-[var(--color-danger)]"
                  : done
                    ? "bg-[var(--color-success)]"
                    : "bg-gradient-to-r from-[var(--color-signal)] to-[var(--color-signal-2)]"
              }`}
              style={{ width: `${failed ? Math.max(pct, 8) : pct}%` }}
            />
            {!failed && !done && pct > 0 && pct < 100 && (
              <div
                className="pointer-events-none absolute inset-y-0 w-24 animate-[update-shimmer_1.6s_ease_infinite] bg-gradient-to-r from-transparent via-white/25 to-transparent"
                style={{ left: `calc(${pct}% - 3rem)` }}
              />
            )}
          </div>
          <div className="mt-1.5 flex justify-between font-mono text-[10px] text-[var(--color-muted)]">
            <span>0</span>
            <span>25</span>
            <span>50</span>
            <span>75</span>
            <span>100</span>
          </div>

          <ol className="mt-4 space-y-1.5">
            {stepStates.map(({ phase, state }) => (
              <li key={phase} className="flex items-center gap-2.5 text-xs">
                <span
                  className={`flex size-5 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold ${
                    state === "done"
                      ? "bg-[var(--color-success-dim)] text-[var(--color-success)]"
                      : state === "active"
                        ? "bg-[var(--color-signal-dim)] text-[var(--color-signal-2)] ring-1 ring-[var(--color-signal)]/40"
                        : state === "failed"
                          ? "bg-[var(--color-danger-dim)] text-[var(--color-danger)]"
                          : "bg-[var(--color-panel-3)] text-[var(--color-muted)]"
                  }`}
                >
                  {state === "done" ? "✓" : state === "failed" ? "×" : PHASE_ORDER.indexOf(phase) + 1}
                </span>
                <span
                  className={
                    state === "active"
                      ? "font-medium text-[var(--color-snow)]"
                      : state === "done"
                        ? "text-[var(--color-fog)]"
                        : "text-[var(--color-muted)]"
                  }
                >
                  {t(`admin.updatesPhase.${phase}`)}
                </span>
              </li>
            ))}
          </ol>

          {progress?.updatedAt && (
            <div className="mt-3 text-[10px] text-[var(--color-muted)]">
              {t("admin.updatesProgressUpdated")}: {progress.updatedAt}
            </div>
          )}
        </div>
      )}

      {(error || info?.error || progress?.error) && (
        <div className="text-sm text-[var(--color-warn)]">
          {error || info?.error || progress?.error}
        </div>
      )}
      {msg && <div className="text-sm text-[var(--color-signal-2)]">{msg}</div>}

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          disabled={!canApply}
          className="btn-primary"
          title={
            info?.updatePending
              ? t("admin.updatesPending")
              : !info?.updateAvailable && !info?.pendingStale
                ? t("admin.updatesNone")
                : undefined
          }
          onClick={async () => {
            if (!accessToken || !canApply) return;
            if (!window.confirm(t("admin.updatesConfirm"))) return;
            setBusy(true);
            setMsg(null);
            setError(null);
            try {
              const res = await api.applyUpdate(accessToken, {
                force:
                  !!info?.pendingStale ||
                  (!!info?.progress?.done &&
                    (info.progress.phase === "failed" || !!info.progress.error)),
              });
              setMsg(res.message);
              setSmoothPct(2);
              await refresh();
            } catch (err) {
              setError(err instanceof Error ? err.message : "error");
            } finally {
              setBusy(false);
            }
          }}
        >
          {busy ? t("common.loading") : t("admin.updatesApply")}
        </button>

        {canForce && (
          <button
            type="button"
            className="btn-ghost"
            disabled={busy || !accessToken}
            title={t("admin.updatesForceHint")}
            onClick={async () => {
              if (!accessToken) return;
              if (!window.confirm(t("admin.updatesForceConfirm"))) return;
              setBusy(true);
              setMsg(null);
              setError(null);
              try {
                const res = await api.applyUpdate(accessToken, { force: true });
                setMsg(res.message);
                setSmoothPct(2);
                await refresh();
              } catch (err) {
                setError(err instanceof Error ? err.message : "error");
              } finally {
                setBusy(false);
              }
            }}
          >
            {t("admin.updatesForce")}
          </button>
        )}

        {(info?.updatePending ||
          info?.pendingStale ||
          (progress?.done && failed) ||
          (progress?.done && done)) && (
          <button
            type="button"
            className="btn-ghost"
            disabled={busy || !accessToken}
            onClick={async () => {
              if (!accessToken) return;
              setBusy(true);
              setError(null);
              try {
                const res = await api.clearUpdate(accessToken);
                setMsg(res.message);
                setSmoothPct(0);
                await refresh();
              } catch (err) {
                setError(err instanceof Error ? err.message : "error");
              } finally {
                setBusy(false);
              }
            }}
          >
            {done ? t("admin.updatesDismissProgress") : t("admin.updatesClearPending")}
          </button>
        )}
      </div>
    </div>
  );
}
