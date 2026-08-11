import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, type SystemHealth } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Badge, Button, Skeleton } from "./ui";
import { IconRefresh } from "./ui/icons";

export function AdminHealthPanel() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [health, setHealth] = useState<SystemHealth | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const h = await api.systemHealth(accessToken);
      setHealth(h);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken]);

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), 15000);
    return () => window.clearInterval(id);
  }, [refresh]);

  async function onRefresh() {
    setBusy(true);
    await refresh();
    setBusy(false);
  }

  const checks = health
    ? ([
        ["postgres", health.checks.postgres.ok],
        ["redis", health.checks.redis.ok],
        ["docker", health.checks.docker.ok],
        ["traefik", health.checks.traefik.ok],
      ] as const)
    : [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-[var(--color-muted)]">{t("admin.healthHint")}</p>
        <Button variant="ghost" className="!py-1.5 !text-xs" disabled={busy} onClick={() => void onRefresh()}>
          <IconRefresh size={14} />
          {t("common.refresh")}
        </Button>
      </div>

      {error && <Alert tone="danger">{error}</Alert>}

      {!health && !error ? (
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {checks.map(([name, ok]) => (
            <div
              key={name}
              className="rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-3 py-3"
            >
              <div className="text-xs uppercase tracking-wider text-[var(--color-muted)]">{name}</div>
              <div className="mt-2">
                <Badge tone={ok ? "success" : "danger"}>
                  {ok ? t("admin.healthOk") : t("admin.healthBad")}
                </Badge>
              </div>
              {name === "traefik" && health?.checks.traefik.status && (
                <div className="mt-1 truncate font-mono text-[10px] text-[var(--color-fog)]">
                  {health.checks.traefik.status}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {health && (
        <p className="font-mono text-xs text-[var(--color-muted)]">
          {health.status} · {health.checkedAt}
        </p>
      )}
    </div>
  );
}
