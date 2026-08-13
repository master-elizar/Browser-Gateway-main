import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, type TIKeyView } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Button, Field, Input } from "./ui";
import { useToast } from "./ui/Toast";

// Same provider set/order as backend/internal/handlers/account.go's accountTIKeyProviders.
const PROVIDERS = ["virustotal", "threatfox", "abuseipdb", "otx", "shodan", "safebrowsing", "malwarebazaar"];

export function ApiKeysSection() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const toast = useToast();
  const [items, setItems] = useState<Record<string, TIKeyView>>({});
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!accessToken) return;
    let cancelled = false;
    void api
      .listTIKeys(accessToken)
      .then((res) => {
        if (cancelled) return;
        const byProvider: Record<string, TIKeyView> = {};
        for (const it of res.items) byProvider[it.provider] = it;
        setItems(byProvider);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "error");
      });
    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  async function save(provider: string) {
    if (!accessToken) return;
    const key = (drafts[provider] || "").trim();
    if (!key) return;
    setBusy(provider);
    setError(null);
    try {
      const res = await api.setTIKey(accessToken, provider, key);
      setItems((prev) => ({ ...prev, [provider]: { provider, keySet: res.keySet, keyMasked: res.keyMasked } }));
      setDrafts((prev) => ({ ...prev, [provider]: "" }));
      toast.push(t("account.apiKeySaved"), "success");
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("account.apiKeyError");
      setError(msg);
      toast.push(msg, "danger");
    } finally {
      setBusy(null);
    }
  }

  async function clear(provider: string) {
    if (!accessToken) return;
    setBusy(provider);
    setError(null);
    try {
      await api.deleteTIKey(accessToken, provider);
      setItems((prev) => ({ ...prev, [provider]: { provider, keySet: false } }));
      toast.push(t("account.apiKeyCleared"), "success");
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("account.apiKeyError");
      setError(msg);
      toast.push(msg, "danger");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <p className="text-xs text-[var(--color-fog)]">{t("account.apiKeysHint")}</p>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="divide-y divide-[var(--color-line)]">
        {PROVIDERS.map((provider) => {
          const view = items[provider];
          const label = t(`account.tiProvider.${provider}`);
          return (
            <div key={provider} className="flex flex-wrap items-end gap-3 py-3 first:pt-0 last:pb-0">
              <div className="min-w-0 flex-1">
                <Field
                  label={label}
                  hint={view?.keySet ? t("account.apiKeySetHint") : t("account.apiKeyUnsetHint")}
                >
                  <Input
                    className="font-mono text-sm"
                    type="password"
                    autoComplete="off"
                    placeholder={view?.keySet ? view.keyMasked || "••••••••" : ""}
                    value={drafts[provider] || ""}
                    onChange={(e) => setDrafts((prev) => ({ ...prev, [provider]: e.target.value }))}
                    spellCheck={false}
                  />
                </Field>
              </div>
              <Button
                type="button"
                variant="ghost"
                disabled={busy === provider || !(drafts[provider] || "").trim()}
                onClick={() => void save(provider)}
              >
                {t("common.save")}
              </Button>
              {view?.keySet && (
                <Button type="button" variant="danger" disabled={busy === provider} onClick={() => void clear(provider)}>
                  {t("account.apiKeyClear")}
                </Button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
