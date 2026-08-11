import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type BrowserSession, type CreateSessionInput } from "../api/client";
import { useAuth, isNotImplemented } from "../auth/AuthContext";
import { LaunchConstructor } from "../components/LaunchConstructor";
import {
  Alert,
  Badge,
  Button,
  EmptyState,
  PageHeader,
  Skeleton,
} from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty, IconPlus } from "../components/ui/icons";
import { useToast } from "../components/ui/Toast";

function statusTone(status: string): "success" | "warn" | "danger" | "neutral" | "accent" {
  const s = status.toLowerCase();
  if (s.includes("run")) return "success";
  if (s.includes("err") || s.includes("fail")) return "danger";
  if (s.includes("pend") || s.includes("creat") || s.includes("start")) return "warn";
  if (s.includes("stop") || s.includes("end")) return "neutral";
  return "accent";
}

export function SessionsPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const toast = useToast();
  const [items, setItems] = useState<BrowserSession[]>([]);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [wizardOpen, setWizardOpen] = useState(false);

  const refresh = useCallback(async () => {
    if (!accessToken) {
      setItems([]);
      setLoading(false);
      return;
    }
    try {
      const res = await api.listSessions(accessToken);
      setItems(res.items ?? []);
      setNote(null);
      setError(null);
    } catch (err) {
      if (isNotImplemented(err)) setNote(t("sessions.stageNote"));
      else setError(err instanceof Error ? err.message : "error");
    } finally {
      setLoading(false);
    }
  }, [accessToken, t]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function launch(input: CreateSessionInput) {
    setBusy(true);
    setError(null);
    try {
      if (!accessToken) return;
      const session = await api.createSession(accessToken, input);
      setItems((prev) => [session, ...prev]);
      setWizardOpen(false);
      toast.push(t("sessions.launch"), "success");
    } catch (err) {
      if (isNotImplemented(err)) setNote(t("sessions.stageNote"));
      else {
        const msg = err instanceof Error ? err.message : t("sessions.createError");
        setError(msg);
        toast.push(msg, "danger");
      }
    } finally {
      setBusy(false);
    }
  }

  async function stop(id: string) {
    if (!accessToken) return;
    try {
      await api.stopSession(accessToken, id);
      await refresh();
      toast.push(t("sessions.stop"), "info");
    } catch (err) {
      if (isNotImplemented(err)) setNote(t("sessions.stageNote"));
      else setError(err instanceof Error ? err.message : "error");
    }
  }

  const openWizard = () => setWizardOpen(true);

  return (
    <div className="animate-fade-in">
      <PageHeader
        title={t("sessions.title")}
        subtitle={t("sessions.subtitle")}
        actions={
          <Button disabled={busy} onClick={openWizard}>
            <IconPlus size={16} />
            {t("sessions.launch")}
          </Button>
        }
      />

      {note && (
        <div className="mb-4">
          <Alert tone="info">{note}</Alert>
        </div>
      )}
      {error && (
        <div className="mb-4">
          <Alert tone="danger">{error}</Alert>
        </div>
      )}

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
              <th>{t("sessions.started")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr>
                <td colSpan={5} className="!p-0">
                  <EmptyState
                    icon={<IconEmpty />}
                    title={t("sessions.empty")}
                    description={t("sessions.subtitle")}
                    action={
                      <Button disabled={busy} onClick={openWizard}>
                        <IconPlus size={16} />
                        {t("sessions.launch")}
                      </Button>
                    }
                  />
                </td>
              </tr>
            ) : (
              items.map((s) => (
                <tr key={s.id}>
                  <td>
                    <div className="font-medium text-[var(--color-snow)]">{s.name || s.id}</div>
                    <div className="mt-0.5 font-mono text-[10px] text-[var(--color-muted)]">
                      {s.id.slice(0, 8)}
                      {s.memoryMb ? ` · ${s.memoryMb}MB` : ""}
                      {s.cpus ? ` · ${s.cpus} CPU` : ""}
                    </div>
                  </td>
                  <td>
                    <Badge tone="neutral">{s.browser || "chromium"}</Badge>
                  </td>
                  <td>
                    <Badge tone={statusTone(s.status)}>{s.status}</Badge>
                  </td>
                  <td className="text-[var(--color-fog)]">
                    {s.startedAt || s.createdAt
                      ? new Date(s.startedAt || s.createdAt || "").toLocaleString()
                      : "—"}
                  </td>
                  <td className="text-right">
                    <div className="flex justify-end gap-2">
                      <Link to={`/sessions/${s.id}`} className="btn-ghost !py-1.5 !text-xs">
                        {t("sessions.open")}
                      </Link>
                      <Button
                        variant="ghost"
                        className="!py-1.5 !text-xs"
                        onClick={() => void stop(s.id)}
                      >
                        {t("sessions.stop")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </DataTable>
      )}

      {accessToken && (
        <LaunchConstructor
          open={wizardOpen}
          busy={busy}
          accessToken={accessToken}
          onClose={() => setWizardOpen(false)}
          onLaunch={(input) => void launch(input)}
        />
      )}
    </div>
  );
}
