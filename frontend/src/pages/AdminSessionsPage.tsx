import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type BrowserSession } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { usePolling } from "../hooks/usePolling";
import { statusTone, hasTransientStatus } from "../lib/sessionStatus";
import { Alert, Badge, Button, EmptyState, PageHeader, Skeleton } from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty, IconRefresh } from "../components/ui/icons";

export function AdminSessionsPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [items, setItems] = useState<BrowserSession[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (!accessToken) {
      setLoading(false);
      return;
    }
    try {
      const res = await api.listAdminSessions(accessToken);
      setItems(res.items);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setLoading(false);
    }
  }, [accessToken]);

  usePolling(refresh, () => (hasTransientStatus(items) ? 3000 : 15000), {
    enabled: Boolean(accessToken),
  });

  async function stop(id: string) {
    if (!accessToken) return;
    try {
      await api.adminStopSession(accessToken, id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  return (
    <div className="animate-fade-in space-y-6">
      <PageHeader
        title={t("admin.sessionsTitle")}
        subtitle={t("admin.sessionsSubtitle")}
        actions={
          <Button variant="ghost" onClick={() => void refresh()}>
            <IconRefresh size={16} />
            {t("common.refresh")}
          </Button>
        }
      />

      {error && <Alert tone="danger">{error}</Alert>}

      {loading ? (
        <div className="ui-card space-y-3 p-5">
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-2/3" />
        </div>
      ) : (
      <DataTable>
        <thead>
          <tr>
            <th>{t("sessions.name")}</th>
            <th>{t("sessions.browser")}</th>
            <th>{t("sessions.status")}</th>
            <th>{t("admin.owner")}</th>
            <th>{t("sessions.started")}</th>
            <th>{t("admin.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr>
              <td colSpan={6} className="!p-0">
                <EmptyState icon={<IconEmpty />} title={t("sessions.empty")} />
              </td>
            </tr>
          ) : (
            items.map((s) => (
              <tr key={s.id}>
                <td>
                  <div className="font-medium">{s.name}</div>
                  <div className="font-mono text-[10px] text-[var(--color-muted)]">{s.id.slice(0, 8)}</div>
                </td>
                <td>
                  <Badge tone="neutral">{s.browser || "chromium"}</Badge>
                </td>
                <td>
                  <Badge tone={statusTone(s.status)}>{s.status}</Badge>
                </td>
                <td className="font-mono text-xs text-[var(--color-fog)]">
                  {s.ownerId?.slice(0, 8) || "—"}
                </td>
                <td className="text-[var(--color-fog)]">
                  {s.startedAt ? new Date(s.startedAt).toLocaleString() : "—"}
                </td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <Link to={`/sessions/${s.id}`} className="btn-ghost !min-h-0 !py-1.5 !text-xs">
                      {t("sessions.open")}
                    </Link>
                    {s.status === "RUNNING" || s.status === "STARTING" || s.status === "IDLE" ? (
                      <Button variant="danger" className="!min-h-0 !py-1.5 !text-xs" onClick={() => void stop(s.id)}>
                        {t("sessions.stop")}
                      </Button>
                    ) : null}
                  </div>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </DataTable>
      )}
    </div>
  );
}
