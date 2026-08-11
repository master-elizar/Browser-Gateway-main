import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type HistoryListItem } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Badge, Button, EmptyState, Field, Input, PageHeader, Select, Skeleton } from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty } from "../components/ui/icons";

export function HistoryPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [items, setItems] = useState<HistoryListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [browser, setBrowser] = useState("");
  const [tiVerdict, setTiVerdict] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    setLoading(true);
    try {
      const res = await api.listHistory(accessToken, {
        name: name || undefined,
        browser: browser || undefined,
        tiVerdict: tiVerdict || undefined,
        from: from ? new Date(from).toISOString() : undefined,
        to: to ? new Date(to).toISOString() : undefined,
      });
      setItems(res.items);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setLoading(false);
    }
  }, [accessToken, name, browser, tiVerdict, from, to]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="animate-fade-in space-y-4">
      <PageHeader title={t("history.title")} subtitle={t("history.subtitle")} />
      {error && <Alert tone="warn">{error}</Alert>}

      <div className="ui-card grid gap-3 p-4 md:grid-cols-5">
        <Field label={t("history.filterName")}>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("history.filterNamePh")} />
        </Field>
        <Field label={t("history.filterBrowser")}>
          <Select value={browser} onChange={(e) => setBrowser(e.target.value)}>
            <option value="">{t("history.filterAny")}</option>
            <option value="chromium">Chromium</option>
            <option value="firefox">Firefox</option>
          </Select>
        </Field>
        <Field label={t("history.filterTi")}>
          <Select value={tiVerdict} onChange={(e) => setTiVerdict(e.target.value)}>
            <option value="">{t("history.filterAny")}</option>
            <option value="malicious">malicious</option>
            <option value="suspicious">suspicious</option>
            <option value="harmless">harmless</option>
            <option value="unknown">unknown</option>
          </Select>
        </Field>
        <Field label={t("history.filterFrom")}>
          <Input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />
        </Field>
        <Field label={t("history.filterTo")}>
          <Input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />
        </Field>
        <div className="md:col-span-5">
          <Button type="button" onClick={() => void refresh()}>
            {t("history.applyFilters")}
          </Button>
        </div>
      </div>

      {loading ? (
        <Skeleton className="h-40" />
      ) : items.length === 0 ? (
        <EmptyState icon={<IconEmpty size={28} />} title={t("history.empty")} />
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>{t("history.colName")}</th>
              <th>{t("history.colBrowser")}</th>
              <th>{t("history.colDuration")}</th>
              <th>{t("history.colFrames")}</th>
              <th>{t("history.colTi")}</th>
              <th>{t("history.colStopped")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((it) => (
              <tr key={it.id}>
                <td className="font-medium text-[var(--color-snow)]">{it.name}</td>
                <td className="font-mono text-xs">{it.browser || "—"}</td>
                <td className="font-mono text-xs">
                  {it.durationSec != null ? `${Math.round(it.durationSec / 60)}m` : "—"}
                </td>
                <td className="font-mono text-xs">{it.frameCount}</td>
                <td>
                  {it.tiVerdict ? (
                    <Badge tone={it.tiVerdict === "malicious" ? "danger" : it.tiVerdict === "suspicious" ? "warn" : "neutral"}>
                      {it.tiVerdict}
                    </Badge>
                  ) : (
                    "—"
                  )}
                </td>
                <td className="font-mono text-xs text-[var(--color-fog)]">
                  {(it.stoppedAt || it.createdAt || "").replace("T", " ").slice(0, 19)}
                </td>
                <td>
                  <Link className="text-sm text-[var(--color-signal-2)] hover:underline" to={`/history/${it.id}`}>
                    {t("history.open")}
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      )}
    </div>
  );
}
