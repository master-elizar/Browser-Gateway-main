import { useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import {
  api,
  type CreateSessionInput,
  type LaunchOptions,
} from "../api/client";
import { Button, Field, Input, Select } from "./ui";

type Step = "browser" | "hardware" | "network" | "review";

const STEPS: Step[] = ["browser", "hardware", "network", "review"];

type Draft = CreateSessionInput;

type Props = {
  open: boolean;
  busy?: boolean;
  accessToken: string;
  onClose: () => void;
  onLaunch: (input: CreateSessionInput) => void;
};

export function LaunchConstructor({ open, busy, accessToken, onClose, onLaunch }: Props) {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>("browser");
  const [options, setOptions] = useState<LaunchOptions | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>({
    name: "",
    browser: "chromium",
    startUrl: "",
    dnsMode: "docker",
    dnsServers: "",
    dnsDohUrl: "",
    memoryMb: 1536,
    cpus: 1.5,
    resolution: "1280x800x24",
    networkEventLimit: 500,
  });
  const [nameExample, setNameExample] = useState(() => `Session ${new Date().toLocaleTimeString()}`);
  const [urlExample, setUrlExample] = useState("https://example.com");
  const [dnsServersExample, setDnsServersExample] = useState("8.8.8.8,1.1.1.1");
  const [dnsDohExample, setDnsDohExample] = useState("https://cloudflare-dns.com/dns-query");

  useEffect(() => {
    if (!open) return;
    setStep("browser");
    setLoadError(null);
    const suggestedName = `Session ${new Date().toLocaleTimeString()}`;
    setNameExample(suggestedName);
    let cancelled = false;
    (async () => {
      try {
        const opts = await api.launchOptions(accessToken);
        if (cancelled) return;
        setOptions(opts);
        setUrlExample(opts.defaults.startUrl || "https://example.com");
        setDnsServersExample(opts.defaults.dnsServers || "8.8.8.8,1.1.1.1");
        setDnsDohExample(opts.defaults.dnsDohUrl || "https://cloudflare-dns.com/dns-query");
        setDraft({
          name: "",
          browser: opts.defaults.browser || "chromium",
          startUrl: "",
          dnsMode: opts.defaults.dnsMode || "docker",
          dnsServers: "",
          dnsDohUrl: "",
          memoryMb: opts.defaults.memoryMb,
          cpus: opts.defaults.cpus,
          resolution: opts.defaults.resolution,
          networkEventLimit: opts.defaults.networkEventLimit || 500,
        });
      } catch (err) {
        if (!cancelled) setLoadError(err instanceof Error ? err.message : "error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, accessToken]);

  const stepIndex = STEPS.indexOf(step);
  const limits = options?.limits;
  const browsers = options?.browsers ?? [
    { id: "chromium", name: "Chromium", description: "" },
    { id: "firefox", name: "Firefox", description: "" },
  ];

  const summary = useMemo(() => {
    const b = browsers.find((x) => x.id === draft.browser)?.name || draft.browser;
    return {
      browser: b,
      memory: `${draft.memoryMb} MB`,
      cpus: `${draft.cpus}`,
      resolution: draft.resolution,
      dns: draft.dnsMode,
      url: draft.startUrl || urlExample,
      name: draft.name || nameExample,
      dnsServers: draft.dnsServers || dnsServersExample,
      dnsDohUrl: draft.dnsDohUrl || dnsDohExample,
      networkEventLimit:
        draft.networkEventLimit === -1
          ? t("launch.networkEventLimitUnlimited")
          : `${draft.networkEventLimit ?? 500}`,
    };
  }, [browsers, draft, urlExample, nameExample, dnsServersExample, dnsDohExample, t]);

  function resolvedLaunchInput(): CreateSessionInput {
    return {
      ...draft,
      name: (draft.name ?? "").trim() || nameExample,
      startUrl: (draft.startUrl ?? "").trim() || urlExample,
      dnsServers: (draft.dnsServers ?? "").trim() || dnsServersExample,
      dnsDohUrl: (draft.dnsDohUrl ?? "").trim() || dnsDohExample,
    };
  }

  if (!open) return null;

  function goNext() {
    const i = STEPS.indexOf(step);
    if (i < STEPS.length - 1) setStep(STEPS[i + 1]);
  }
  function goBack() {
    const i = STEPS.indexOf(step);
    if (i > 0) setStep(STEPS[i - 1]);
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <button
        type="button"
        className="absolute inset-0 bg-black/55 backdrop-blur-[2px]"
        aria-label="Close"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        className="relative z-10 flex h-[min(720px,92vh)] w-full max-w-4xl overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-line)] bg-[var(--color-panel)] shadow-[var(--shadow-md)]"
      >
        {/* vSphere-like left rail */}
        <aside className="hidden w-56 shrink-0 border-r border-[var(--color-line)] bg-[var(--color-bg-elevated)] md:flex md:flex-col">
          <div className="border-b border-[var(--color-line)] px-4 py-4">
            <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--color-muted)]">
              {t("launch.wizardLabel")}
            </div>
            <div className="mt-1 text-sm font-semibold text-[var(--color-snow)]">{t("launch.title")}</div>
          </div>
          <nav className="flex flex-1 flex-col gap-0.5 p-2">
            {STEPS.map((id, idx) => {
              const active = id === step;
              const done = idx < stepIndex;
              return (
                <button
                  key={id}
                  type="button"
                  onClick={() => setStep(id)}
                  className={`flex items-start gap-3 rounded-[var(--radius-sm)] px-3 py-2.5 text-left transition ${
                    active
                      ? "bg-[var(--color-signal-dim)] text-[var(--color-snow)]"
                      : "text-[var(--color-fog)] hover:bg-[var(--color-surface-hover)]"
                  }`}
                >
                  <span
                    className={`mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold ${
                      active
                        ? "bg-[var(--color-signal)] text-[var(--color-ink)]"
                        : done
                          ? "bg-[var(--color-success-dim)] text-[var(--color-success)]"
                          : "bg-[var(--color-panel-3)] text-[var(--color-muted)]"
                    }`}
                  >
                    {idx + 1}
                  </span>
                  <span>
                    <span className="block text-xs font-medium">{t(`launch.steps.${id}`)}</span>
                    <span className="mt-0.5 block text-[10px] text-[var(--color-muted)]">
                      {t(`launch.steps.${id}Hint`)}
                    </span>
                  </span>
                </button>
              );
            })}
          </nav>
        </aside>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex items-center justify-between border-b border-[var(--color-line)] px-5 py-3.5">
            <div>
              <h2 className="text-base font-semibold text-[var(--color-snow)]">{t(`launch.steps.${step}`)}</h2>
              <p className="mt-0.5 text-xs text-[var(--color-muted)]">{t(`launch.steps.${step}Hint`)}</p>
            </div>
            <Button variant="ghost" className="!min-h-0 !px-2 !py-1 text-xs" onClick={onClose}>
              {t("launch.cancel")}
            </Button>
          </header>

          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
            {loadError && (
              <div className="mb-3 rounded-[var(--radius-sm)] border border-[var(--color-danger)]/30 bg-[var(--color-danger-dim)] px-3 py-2 text-xs text-[var(--color-danger)]">
                {loadError}
              </div>
            )}

            {step === "browser" && (
              <div className="space-y-4">
                <Field label={t("launch.name")}>
                  <Input
                    value={draft.name}
                    onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
                    placeholder={nameExample}
                  />
                </Field>
                <Field label={t("launch.startUrl")}>
                  <Input
                    className="font-mono text-sm"
                    value={draft.startUrl}
                    onChange={(e) => setDraft((d) => ({ ...d, startUrl: e.target.value }))}
                    placeholder={urlExample}
                    spellCheck={false}
                  />
                </Field>
                <div>
                  <div className="field-label mb-2">{t("launch.selectBrowser")}</div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    {browsers.map((b) => {
                      const selected = draft.browser === b.id;
                      return (
                        <button
                          key={b.id}
                          type="button"
                          onClick={() => setDraft((d) => ({ ...d, browser: b.id }))}
                          className={`rounded-[var(--radius-md)] border px-4 py-4 text-left transition ${
                            selected
                              ? "border-[var(--color-signal)] bg-[var(--color-signal-dim)]"
                              : "border-[var(--color-line)] bg-[var(--color-surface)] hover:border-[var(--color-line-strong)]"
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-semibold text-[var(--color-snow)]">{b.name}</span>
                            <span
                              className={`size-2.5 rounded-full ${
                                selected ? "bg-[var(--color-signal)]" : "bg-[var(--color-panel-3)]"
                              }`}
                            />
                          </div>
                          <p className="mt-1.5 text-xs leading-relaxed text-[var(--color-muted)]">
                            {b.description || t(`launch.browserDesc.${b.id}`, { defaultValue: "" })}
                          </p>
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}

            {step === "hardware" && (
              <div className="space-y-4">
                <Field
                  label={`${t("launch.memory")} (${draft.memoryMb} MB)`}
                  hint={t("launch.memoryHint")}
                >
                  <input
                    type="range"
                    min={limits?.memoryMbMin ?? 512}
                    max={limits?.memoryMbMax ?? 8192}
                    step={256}
                    value={draft.memoryMb ?? 1536}
                    onChange={(e) => setDraft((d) => ({ ...d, memoryMb: Number(e.target.value) }))}
                    className="w-full accent-[var(--color-signal)]"
                  />
                  <div className="mt-1 flex justify-between text-[10px] text-[var(--color-muted)]">
                    <span>{limits?.memoryMbMin ?? 512} MB</span>
                    <span>{limits?.memoryMbMax ?? 8192} MB</span>
                  </div>
                </Field>
                <Field label={`${t("launch.cpus")} (${draft.cpus})`} hint={t("launch.cpusHint")}>
                  <input
                    type="range"
                    min={limits?.cpusMin ?? 0.5}
                    max={limits?.cpusMax ?? 8}
                    step={0.5}
                    value={draft.cpus ?? 1.5}
                    onChange={(e) => setDraft((d) => ({ ...d, cpus: Number(e.target.value) }))}
                    className="w-full accent-[var(--color-signal)]"
                  />
                  <div className="mt-1 flex justify-between text-[10px] text-[var(--color-muted)]">
                    <span>{limits?.cpusMin ?? 0.5}</span>
                    <span>{limits?.cpusMax ?? 8}</span>
                  </div>
                </Field>
                <Field label={t("launch.resolution")} hint={t("launch.resolutionHint")}>
                  <Select
                    value={draft.resolution || "1280x800x24"}
                    onChange={(e) => setDraft((d) => ({ ...d, resolution: e.target.value }))}
                  >
                    {(limits?.resolutions ?? ["1280x800x24", "1920x1080x24"]).map((r) => (
                      <option key={r} value={r}>
                        {r.replace(/x24$/, "")}
                      </option>
                    ))}
                  </Select>
                </Field>
              </div>
            )}

            {step === "network" && (
              <div className="space-y-4">
                <Field label={t("admin.dnsMode")} hint={t("launch.dnsHint")}>
                  <Select
                    value={draft.dnsMode || "docker"}
                    onChange={(e) => setDraft((d) => ({ ...d, dnsMode: e.target.value }))}
                  >
                    <option value="docker">{t("admin.dnsModeDocker")}</option>
                    <option value="custom">{t("admin.dnsModeCustom")}</option>
                    <option value="doh">{t("admin.dnsModeDoh")}</option>
                    <option value="custom_doh">{t("admin.dnsModeCustomDoh")}</option>
                  </Select>
                </Field>
                <Field label={t("admin.dnsServers")}>
                  <Input
                    className="font-mono text-sm"
                    value={draft.dnsServers || ""}
                    onChange={(e) => setDraft((d) => ({ ...d, dnsServers: e.target.value }))}
                    placeholder={dnsServersExample}
                    spellCheck={false}
                  />
                </Field>
                <Field label={t("admin.dnsDohUrl")}>
                  <Input
                    className="font-mono text-sm"
                    value={draft.dnsDohUrl || ""}
                    onChange={(e) => setDraft((d) => ({ ...d, dnsDohUrl: e.target.value }))}
                    placeholder={dnsDohExample}
                    spellCheck={false}
                  />
                </Field>
                <Field label={t("launch.networkEventLimit")} hint={t("launch.networkEventLimitHint")}>
                  <Select
                    value={String(draft.networkEventLimit ?? 500)}
                    onChange={(e) => setDraft((d) => ({ ...d, networkEventLimit: Number(e.target.value) }))}
                  >
                    {(limits?.networkEventLimits ?? [200, 500, 1000, 2000, 5000, -1]).map((n) => (
                      <option key={n} value={n}>
                        {n === -1 ? t("launch.networkEventLimitUnlimited") : n}
                      </option>
                    ))}
                  </Select>
                </Field>
              </div>
            )}

            {step === "review" && (
              <div className="space-y-4">
                <p className="text-xs text-[var(--color-muted)]">{t("launch.reviewHint")}</p>
                <dl className="divide-y divide-[var(--color-line)] rounded-[var(--radius-md)] border border-[var(--color-line)]">
                  {[
                    [t("launch.name"), summary.name],
                    [t("launch.selectBrowser"), summary.browser],
                    [t("launch.startUrl"), summary.url],
                    [t("launch.memory"), summary.memory],
                    [t("launch.cpus"), summary.cpus],
                    [t("launch.resolution"), summary.resolution],
                    [t("admin.dnsMode"), summary.dns],
                    [t("admin.dnsServers"), summary.dnsServers],
                    [t("admin.dnsDohUrl"), summary.dnsDohUrl],
                    [t("launch.networkEventLimit"), summary.networkEventLimit],
                  ].map(([k, v]) => (
                    <div key={String(k)} className="flex gap-4 px-4 py-2.5 text-xs">
                      <dt className="shrink-0 text-[var(--color-muted)]">{k}</dt>
                      <dd className="min-w-0 flex-1 break-all font-mono text-[var(--color-snow)]">{v || "—"}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            )}
          </div>

          <footer className="flex items-center justify-between gap-3 border-t border-[var(--color-line)] px-5 py-3">
            <Button variant="ghost" disabled={stepIndex === 0 || busy} onClick={goBack}>
              {t("launch.back")}
            </Button>
            <div className="flex gap-2">
              {step !== "review" ? (
                <Button disabled={busy} onClick={goNext}>
                  {t("launch.next")}
                </Button>
              ) : (
                <Button
                  disabled={busy}
                  onClick={() => onLaunch(resolvedLaunchInput())}
                >
                  {t("launch.finish")}
                </Button>
              )}
            </div>
          </footer>
        </div>
      </div>
    </div>,
    document.body,
  );
}
