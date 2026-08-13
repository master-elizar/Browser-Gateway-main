import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api, type DomainCheckResult, type TIResult } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Badge, Button, Card, CardBody, CardHeader, Field, Input, PageHeader, Segmented } from "../components/ui";

type Mode = "simple" | "advanced";
type Tier = "safe" | "suspicious" | "malicious" | "unknown";

function tierTone(tier: Tier): "success" | "warn" | "danger" | "neutral" {
  switch (tier) {
    case "safe":
      return "success";
    case "suspicious":
      return "warn";
    case "malicious":
      return "danger";
    default:
      return "neutral";
  }
}

function verdictTone(verdict: string): "success" | "warn" | "danger" | "neutral" {
  switch (verdict) {
    case "clean":
      return "success";
    case "suspicious":
      return "warn";
    case "malicious":
      return "danger";
    default:
      return "neutral";
  }
}

// Malicious/safe tiers describe the malicious-count sentence ("X sources consider this
// domain malicious"); the suspicious tier describes a broader flagged-count sentence
// ("X sources flag this domain as suspicious") -- see domainCheckTier on the backend for
// why flaggedCount itself differs per tier.
function verdictWord(tier: Tier): "malicious" | "suspicious" {
  return tier === "suspicious" ? "suspicious" : "malicious";
}

function formatDate(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString();
}

export function DomainCheckPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [value, setValue] = useState("");
  const [mode, setMode] = useState<Mode>("simple");
  const [result, setResult] = useState<DomainCheckResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!accessToken || !value.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const res = await api.checkDomain(accessToken, value.trim());
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("domainCheck.error"));
      setResult(null);
    } finally {
      setBusy(false);
    }
  }

  const tier = (result?.tier ?? "unknown") as Tier;
  const flaggedCount =
    tier === "malicious" ? (result?.malicious ?? 0) : tier === "suspicious" ? (result?.malicious ?? 0) + (result?.suspicious ?? 0) : 0;

  return (
    <div className="mx-auto max-w-3xl animate-fade-in space-y-6">
      <PageHeader title={t("domainCheck.title")} subtitle={t("domainCheck.subtitle")} />

      <Card>
        <CardBody>
          <form onSubmit={onSubmit} className="flex flex-wrap items-end gap-3">
            <div className="min-w-0 flex-1">
              <Field label={t("domainCheck.inputLabel")}>
                <Input
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  placeholder={t("domainCheck.inputPlaceholder")}
                  spellCheck={false}
                  autoComplete="off"
                  required
                />
              </Field>
            </div>
            <Button type="submit" disabled={busy || !value.trim()}>
              {busy ? t("domainCheck.checking") : t("domainCheck.checkButton")}
            </Button>
          </form>
        </CardBody>
      </Card>

      {error && <Alert tone="danger">{error}</Alert>}

      {result && (
        <>
          <Card>
            <CardBody className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <div className="mb-2">
                  <Badge tone={tierTone(tier)}>{t(`domainCheck.tier.${tier}`)}</Badge>
                </div>
                {tier === "unknown" ? (
                  <p className="text-sm text-[var(--color-fog)]">{t("domainCheck.insufficientData")}</p>
                ) : (
                  <p className="text-sm text-[var(--color-fog)]">
                    {t("domainCheck.verdictSentence", {
                      count: flaggedCount,
                      total: result.total,
                      word: t(`domainCheck.word.${verdictWord(tier)}`),
                    })}
                  </p>
                )}
                {result.cached && <p className="mt-1 text-xs text-[var(--color-muted)]">{t("domainCheck.cachedNote")}</p>}
              </div>
              <Segmented
                options={[
                  { value: "simple", label: t("domainCheck.modeSimple") },
                  { value: "advanced", label: t("domainCheck.modeAdvanced") },
                ]}
                value={mode}
                onChange={(v) => setMode(v as Mode)}
              />
            </CardBody>
          </Card>

          {mode === "advanced" && (
            <>
              <Card>
                <CardHeader title={t("domainCheck.whoisTitle")} />
                <CardBody>
                  {result.whois ? (
                    <dl className="divide-y divide-[var(--color-line)]">
                      <div className="flex items-center justify-between gap-4 py-2 first:pt-0">
                        <dt className="text-sm text-[var(--color-fog)]">{t("domainCheck.whoisRegistrar")}</dt>
                        <dd className="text-sm text-[var(--color-snow)]">{result.whois.registrar || "—"}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4 py-2">
                        <dt className="text-sm text-[var(--color-fog)]">{t("domainCheck.whoisRegistered")}</dt>
                        <dd className="text-sm text-[var(--color-snow)]">{formatDate(result.whois.registered)}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4 py-2">
                        <dt className="text-sm text-[var(--color-fog)]">{t("domainCheck.whoisExpires")}</dt>
                        <dd className="text-sm text-[var(--color-snow)]">{formatDate(result.whois.expires)}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4 py-2 last:pb-0">
                        <dt className="text-sm text-[var(--color-fog)]">{t("domainCheck.whoisNameservers")}</dt>
                        <dd className="max-w-xs text-right text-sm text-[var(--color-snow)]">
                          {result.whois.nameservers?.length ? result.whois.nameservers.join(", ") : "—"}
                        </dd>
                      </div>
                    </dl>
                  ) : (
                    <p className="text-sm text-[var(--color-muted)]">
                      {result.whoisError || t("domainCheck.whoisUnavailable")}
                    </p>
                  )}
                </CardBody>
              </Card>

              <Card>
                <CardHeader title={t("domainCheck.providersTitle")} />
                <CardBody className="space-y-1">
                  {result.providers.length === 0 ? (
                    <p className="text-sm text-[var(--color-muted)]">{t("domainCheck.providersEmpty")}</p>
                  ) : (
                    <div className="divide-y divide-[var(--color-line)]">
                      {result.providers.map((p: TIResult) => (
                        <div key={p.provider} className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium text-[var(--color-snow)]">{p.provider}</span>
                              {p.informational && <Badge tone="neutral">{t("domainCheck.providerInformational")}</Badge>}
                            </div>
                            <p className="mt-0.5 truncate text-xs text-[var(--color-fog)]">
                              {p.error ? `${t("domainCheck.providerError")}: ${p.error}` : p.detail || "—"}
                            </p>
                          </div>
                          {!p.informational && !p.error && <Badge tone={verdictTone(p.verdict)}>{p.verdict}</Badge>}
                        </div>
                      ))}
                    </div>
                  )}
                </CardBody>
              </Card>
            </>
          )}
        </>
      )}
    </div>
  );
}
