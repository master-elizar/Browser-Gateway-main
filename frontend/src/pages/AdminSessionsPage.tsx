import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type BrowserSession } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { usePolling } from "../hooks/usePolling";
import { Alert, Badge, Button, EmptyState, PageHeader } from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty, IconRefresh } from "../components/ui/icons";

function statusTone(status: string): "success" | "warn" | "danger" | "neutral" | "accent" {
  const s = status.toLowerCase();
  if (s.includes("run")) return "success";
  if (s.includes("err") || s.includes("fail")) return "danger";
  if (s.includes("pend") || s.includes("creat") || s.includes("start") || s.includes("idle")) return "warn";
  if (s.includes("stop") || s.includes("end")) return "neutral";
  return "accent";
}

function hasTransientStatus(items: BrowserSession[]): boolean {
  return items.some((s) => {
    const v = s.status.toLowerCase();
    return v.includes("creat") || v.includes("start") || v.includes("stop");
  });
}

export function AdminSessionsPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [items, setItems] = useState<BrowserSession[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await api.listAdminSessions(accessToken);
      setItems(res.items);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
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
    </div>
  );
}
