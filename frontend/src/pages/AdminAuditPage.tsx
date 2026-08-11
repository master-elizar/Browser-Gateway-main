import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { api, type AuditEvent } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Badge, Button, EmptyState, Input, PageHeader } from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty, IconRefresh } from "../components/ui/icons";

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function formatAuditSummary(t: TFunction, ev: AuditEvent): string {
  const who = ev.userDisplayName || ev.userEmail || ev.userId || t("admin.auditSomeone");
  const typ = ev.type || "";
  if (typ.startsWith("auth.login")) return t("admin.auditLogin", { who });
  if (typ.startsWith("auth.register")) return t("admin.auditRegister", { who });
  if (typ.startsWith("auth.setup")) return t("admin.auditSetup", { who });
  if (typ.startsWith("auth.password")) return t("admin.auditPassword", { who });
  if (typ.includes("session") && typ.includes("create")) return t("admin.auditSessionCreate", { who });
  if (typ.includes("session") && typ.includes("stop")) return t("admin.auditSessionStop", { who });
  if (typ.startsWith("admin.tls")) return t("admin.auditTls", { who });
  if (typ.startsWith("admin.update")) return t("admin.auditUpdate", { who });
  if (typ.startsWith("admin.settings")) return t("admin.auditSettings", { who });
  if (typ.startsWith("admin.user")) return t("admin.auditUserAdmin", { who, detail: ev.message });
  if (ev.message) return t("admin.auditGeneric", { who, detail: ev.message });
  return t("admin.auditFallback", { who, type: typ });
}

export function AdminAuditPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [items, setItems] = useState<AuditEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [typeFilter, setTypeFilter] = useState("");
  const [userFilter, setUserFilter] = useState("");
  const [sessionFilter, setSessionFilter] = useState("");
  const [live, setLive] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exportBusy, setExportBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await api.listAudit(accessToken, {
        type: typeFilter || undefined,
        userId: userFilter || undefined,
        limit: 150,
      });
      let rows = res.items;
      if (sessionFilter.trim()) {
        rows = rows.filter((x) => (x.sessionId || "").includes(sessionFilter.trim()));
      }
      setItems(rows);
      setTotal(res.total);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken, typeFilter, userFilter, sessionFilter]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!live) return;
    const id = window.setInterval(() => void refresh(), 2500);
    return () => window.clearInterval(id);
  }, [live, refresh]);

  async function onExport(format: "csv" | "json" | "rfc5424") {
    if (!accessToken) return;
    setExportBusy(true);
    try {
      const blob = await api.exportAudit(accessToken, format);
      const name =
        format === "rfc5424"
          ? "audit-export.rfc5424.log"
          : format === "json"
            ? "audit-export.json"
            : "audit-export.csv";
      downloadBlob(blob, name);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setExportBusy(false);
    }
  }

  return (
    <div className="animate-fade-in space-y-6">
      <PageHeader
        title={t("admin.auditTitle")}
        subtitle={t("admin.auditSubtitle")}
        actions={
          <>
            <Button variant="ghost" onClick={() => void refresh()}>
              <IconRefresh size={16} />
              {t("common.refresh")}
            </Button>
            <Button variant="ghost" disabled={exportBusy} onClick={() => void onExport("csv")}>
              {t("admin.exportCsv")}
            </Button>
            <Button
              variant="ghost"
              disabled={exportBusy}
              onClick={() => void onExport("rfc5424")}
              title={t("admin.exportRfcHint")}
            >
              {t("admin.exportRfc")}
            </Button>
          </>
        }
      />

      <div className="ui-card flex flex-wrap items-center gap-3 px-4 py-3">
        <Input
          className="!w-44"
          placeholder={t("admin.filterType")}
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
        />
        <Input
          className="!w-44"
          placeholder={t("admin.filterUser")}
          value={userFilter}
          onChange={(e) => setUserFilter(e.target.value)}
        />
        <Input
          className="!w-44"
          placeholder={t("admin.filterSession")}
          value={sessionFilter}
          onChange={(e) => setSessionFilter(e.target.value)}
        />
        <label className="flex items-center gap-2 text-sm text-[var(--color-fog)]">
          <input
            type="checkbox"
            checked={live}
            onChange={(e) => setLive(e.target.checked)}
            className="accent-[var(--color-signal)]"
          />
          {t("admin.live")}
          {live && <Badge tone="success">{t("admin.liveOn")}</Badge>}
        </label>
        <span className="ml-auto text-xs text-[var(--color-muted)]">
          {t("admin.auditTotal")}: {total}
        </span>
      </div>

      {error && <Alert tone="danger">{error}</Alert>}

      <DataTable className="table-fixed">
        <thead>
          <tr>
            <th className="w-[11rem]">{t("admin.when")}</th>
            <th className="w-[14rem]">{t("admin.actor")}</th>
            <th className="w-[10rem]">{t("admin.type")}</th>
            <th>{t("admin.summary")}</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr>
              <td colSpan={4} className="!p-0">
                <EmptyState icon={<IconEmpty />} title={t("admin.auditEmpty")} />
              </td>
            </tr>
          ) : (
            items.map((ev) => (
              <tr key={ev.id}>
                <td className="whitespace-nowrap font-mono text-xs text-[var(--color-fog)]">
                  {new Date(ev.createdAt).toLocaleString()}
                </td>
                <td>
                  <div className="truncate font-medium text-[var(--color-snow)]">
                    {ev.userDisplayName || ev.userEmail || ev.userId || "—"}
                  </div>
                  <div className="truncate text-xs text-[var(--color-muted)]">
                    {[ev.userEmail, ev.userRole].filter(Boolean).join(" · ")}
                  </div>
                </td>
                <td>
                  <Badge tone="accent">{ev.type}</Badge>
                </td>
                <td>
                  <div className="leading-snug text-[var(--color-snow)]">
                    {formatAuditSummary(t, ev) || ev.summary || ev.message}
                  </div>
                  {ev.sessionId && (
                    <div className="mt-1 font-mono text-[10px] text-[var(--color-muted)]">
                      session {ev.sessionId.slice(0, 8)}
                    </div>
                  )}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </DataTable>
    </div>
  );
}
