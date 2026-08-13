import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, type AppSettings } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { Alert, Button, Field, Input, Segmented, Skeleton, Toggle } from "../components/ui";
import { IconBrand, IconCheck } from "../components/ui/icons";

type Step = "admin" | "network" | "limits" | "name" | "review";
type Deployment = "local" | "public";

const STEPS: Step[] = ["admin", "network", "limits", "name", "review"];

type NetworkProgress = {
  percent: number;
  phase: string;
  message: string;
  done: boolean;
  error?: string;
};

export function SetupPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);
  const [step, setStep] = useState<Step>("admin");
  const [maxStepReached, setMaxStepReached] = useState(0);

  // Step 1 — admin account. Kept in local state (not localStorage) until the whole wizard
  // finishes, so this page's own top-of-component `if (user) redirect` guard -- which
  // watches the real, already-logged-in AuthContext state -- can't fire mid-wizard.
  const [setupKey, setSetupKey] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [adminPending, setAdminPending] = useState(false);
  const [adminError, setAdminError] = useState<string | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);

  // Step 2 — network / TURN.
  const [deployment, setDeployment] = useState<Deployment>("local");
  const [turnHosts, setTurnHosts] = useState(() => window.location.hostname || "localhost");
  const [applyNetworkNow, setApplyNetworkNow] = useState(true);

  // Step 3 — session limits (prefilled from current AppSettings once step 1 succeeds).
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [maxGlobal, setMaxGlobal] = useState(15);
  const [maxPerUser, setMaxPerUser] = useState(3);

  // Step 4 — instance name.
  const [instanceName, setInstanceName] = useState("");

  // Step 5 — finish.
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);
  const [networkProgress, setNetworkProgress] = useState<NetworkProgress | null>(null);

  useEffect(() => {
    void api
      .setupStatus()
      .then((s) => setNeedsSetup(s.needsSetup))
      .catch(() => setNeedsSetup(false));
  }, []);

  useEffect(() => {
    if (!accessToken || settings) return;
    void api
      .getSettings(accessToken)
      .then((s) => {
        setSettings(s);
        setMaxGlobal(s.maxConcurrentSessionsGlobal || 15);
        setMaxPerUser(s.maxConcurrentSessionsPerUser || 3);
      })
      .catch(() => undefined);
  }, [accessToken, settings]);

  if (user) return <Navigate to="/sessions" replace />;
  if (needsSetup === false) return <Navigate to="/login" replace />;
  if (needsSetup === null) {
    return (
      <div className="grid min-h-full place-items-center">
        <div className="w-64 space-y-3">
          <Skeleton className="h-8 w-40" />
          <Skeleton className="h-24 w-full" />
        </div>
      </div>
    );
  }

  const stepIndex = STEPS.indexOf(step);

  // The admin step creates the account and can't be meaningfully redone once it has --
  // re-submitting would just fail against a setup key that's already been consumed.
  function stepReachable(target: Step): boolean {
    const targetIndex = STEPS.indexOf(target);
    if (targetIndex > maxStepReached) return false;
    if (target === "admin" && accessToken) return false;
    return true;
  }
  function goTo(target: Step) {
    if (stepReachable(target)) setStep(target);
  }
  function goNext() {
    const i = STEPS.indexOf(step);
    if (i >= STEPS.length - 1) return;
    const next = STEPS[i + 1]!;
    setStep(next);
    setMaxStepReached((m) => Math.max(m, i + 1));
  }
  function goBack() {
    const i = STEPS.indexOf(step);
    if (i <= 0) return;
    const prev = STEPS[i - 1]!;
    if (stepReachable(prev)) setStep(prev);
  }
  const canGoBack = stepIndex > 0 && stepReachable(STEPS[stepIndex - 1]!);

  async function onCreateAdmin(e: FormEvent) {
    e.preventDefault();
    setAdminPending(true);
    setAdminError(null);
    try {
      const pair = await api.setupComplete({ setupKey, email, password, displayName });
      setAccessToken(pair.accessToken);
      setRefreshToken(pair.refreshToken);
      goNext();
    } catch (err) {
      setAdminError(err instanceof Error ? err.message : t("setup.error"));
    } finally {
      setAdminPending(false);
    }
  }

  function turnUrlsValue(): string {
    return turnHosts
      .split(",")
      .map((h) => h.trim())
      .filter(Boolean)
      .map((h) => `turn:${h}:3478`)
      .join(",");
  }

  async function pollNetworkStatus(token: string, deadlineMs: number) {
    const start = Date.now();
    while (Date.now() - start < deadlineMs) {
      try {
        const res = await api.networkStatus(token);
        if (res.progress) setNetworkProgress(res.progress);
        if (res.progress?.done) return;
      } catch {
        // Backend is very likely mid-restart right now -- keep polling instead of failing.
      }
      await new Promise((resolve) => window.setTimeout(resolve, 1500));
    }
  }

  async function finish() {
    if (!accessToken || !refreshToken) return;
    setFinishing(true);
    setFinishError(null);
    try {
      const base = settings ?? (await api.getSettings(accessToken));
      await api.putSettings(accessToken, {
        ...base,
        maxConcurrentSessionsGlobal: maxGlobal,
        maxConcurrentSessionsPerUser: maxPerUser,
        instanceName: instanceName.trim() || undefined,
      });

      if (applyNetworkNow) {
        const turnUrls = turnUrlsValue();
        if (turnUrls) {
          await api.applyNetwork(accessToken, turnUrls);
          await pollNetworkStatus(accessToken, 90_000);
        }
      }

      localStorage.setItem("bg.accessToken", accessToken);
      localStorage.setItem("bg.refreshToken", refreshToken);
      window.location.href = "/sessions";
    } catch (err) {
      setFinishError(err instanceof Error ? err.message : t("setup.error"));
      setFinishing(false);
    }
  }

  return (
    <div className="grid min-h-full place-items-center px-6 py-12">
      <div className="w-full max-w-[480px] animate-fade-in">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-4 grid size-14 place-items-center rounded-[var(--radius-lg)] bg-[var(--color-signal)] text-white shadow-[var(--shadow-md)]">
            <IconBrand size={26} />
          </div>
          <h1 className="text-[1.75rem] font-semibold tracking-tight text-[var(--color-white)]">
            {t("brand")}
          </h1>
          <p className="mt-1 text-sm text-[var(--color-fog)]">{t("setup.subtitle")}</p>
        </div>

        <div className="mb-5 flex items-center justify-center gap-1.5">
          {STEPS.map((s, i) => {
            const active = s === step;
            const reachable = stepReachable(s);
            return (
              <div key={s} className="flex items-center gap-1.5">
                <button
                  type="button"
                  disabled={!reachable}
                  onClick={() => goTo(s)}
                  title={t(`setup.step${cap(s)}`)}
                  className={[
                    "grid size-6 place-items-center rounded-full text-[11px] font-semibold transition",
                    active
                      ? "bg-[var(--color-signal)] text-white"
                      : i < stepIndex
                        ? "bg-[var(--color-success-dim)] text-[var(--color-success)]"
                        : "bg-[var(--color-panel-3)] text-[var(--color-muted)]",
                    reachable && !active ? "cursor-pointer" : "cursor-default",
                  ].join(" ")}
                >
                  {i < stepIndex ? <IconCheck size={12} /> : i + 1}
                </button>
                {i < STEPS.length - 1 && <div className="h-px w-5 bg-[var(--color-line)]" />}
              </div>
            );
          })}
        </div>
        <p className="mb-4 text-center text-xs text-[var(--color-muted)]">
          {t(`setup.step${cap(step)}`)} · {t(`setup.step${cap(step)}Hint`)}
        </p>

        <div className="ui-card p-6">
          {step === "admin" && (
            <form onSubmit={onCreateAdmin} className="space-y-4">
              <Field label={t("setup.key")}>
                <Input
                  className="font-mono"
                  value={setupKey}
                  onChange={(e) => setSetupKey(e.target.value)}
                  placeholder={t("setup.keyPh")}
                  required
                  autoComplete="off"
                />
              </Field>
              <Field label={t("login.displayName")}>
                <Input
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder={t("login.displayNamePh")}
                />
              </Field>
              <Field label={t("login.email")}>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("login.emailPh")}
                  required
                  autoComplete="username"
                />
              </Field>
              <Field label={t("login.password")}>
                <Input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("login.passwordPh")}
                  required
                  minLength={8}
                  autoComplete="new-password"
                />
              </Field>
              {adminError && <Alert tone="warn">{adminError}</Alert>}
              <Button type="submit" disabled={adminPending} className="w-full">
                {adminPending ? t("common.loading") : t("setup.submit")}
              </Button>
            </form>
          )}

          {step === "network" && (
            <div className="space-y-4">
              <div>
                <div className="field-label mb-2">{t("setup.deploymentType")}</div>
                <Segmented
                  fullWidth
                  options={[
                    { value: "local" as const, label: t("setup.deploymentLocal") },
                    { value: "public" as const, label: t("setup.deploymentPublic") },
                  ]}
                  value={deployment}
                  onChange={setDeployment}
                />
              </div>
              <Field label={t("setup.turnHosts")} hint={t("setup.turnHostsHint")}>
                <Input
                  className="font-mono text-sm"
                  value={turnHosts}
                  onChange={(e) => setTurnHosts(e.target.value)}
                  placeholder={t("setup.turnHostsPh")}
                  spellCheck={false}
                />
              </Field>
              {deployment === "public" && <Alert tone="warn">{t("setup.publicWarning")}</Alert>}
            </div>
          )}

          {step === "limits" && (
            <div className="space-y-4">
              <Field label={t("admin.maxGlobal")}>
                <Input
                  type="number"
                  min={1}
                  value={maxGlobal}
                  onChange={(e) => setMaxGlobal(Number(e.target.value) || 1)}
                />
              </Field>
              <Field label={t("admin.maxPerUser")}>
                <Input
                  type="number"
                  min={1}
                  value={maxPerUser}
                  onChange={(e) => setMaxPerUser(Number(e.target.value) || 1)}
                />
              </Field>
            </div>
          )}

          {step === "name" && (
            <div className="space-y-4">
              <Field label={t("setup.instanceName")} hint={t("setup.instanceNameHint")}>
                <Input
                  value={instanceName}
                  onChange={(e) => setInstanceName(e.target.value)}
                  placeholder={t("setup.instanceNamePh")}
                />
              </Field>
            </div>
          )}

          {step === "review" && (
            <div className="space-y-4">
              <p className="text-xs text-[var(--color-muted)]">{t("setup.reviewIntro")}</p>
              <dl className="divide-y divide-[var(--color-line)] rounded-[var(--radius-md)] border border-[var(--color-line)]">
                {[
                  [t("setup.reviewAdmin"), email],
                  [
                    t("setup.reviewDeployment"),
                    deployment === "public" ? t("setup.deploymentPublic") : t("setup.deploymentLocal"),
                  ],
                  [t("setup.reviewTurnHosts"), turnHosts],
                  [t("setup.reviewLimits"), `${maxGlobal} / ${maxPerUser}`],
                  [t("setup.reviewName"), instanceName.trim() || t("setup.instanceNamePh")],
                ].map(([k, v]) => (
                  <div key={String(k)} className="flex gap-4 px-4 py-2.5 text-xs">
                    <dt className="shrink-0 text-[var(--color-muted)]">{k}</dt>
                    <dd className="min-w-0 flex-1 break-all font-mono text-[var(--color-snow)]">{v || "—"}</dd>
                  </div>
                ))}
              </dl>

              <div className="rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-panel-2)] px-3 py-1">
                <Toggle
                  checked={applyNetworkNow}
                  onChange={() => setApplyNetworkNow((v) => !v)}
                  label={t("setup.applyNow")}
                  description={t("setup.applyNowHint")}
                />
              </div>

              {finishing && applyNetworkNow && (
                <Alert tone="info">
                  {networkProgress?.done
                    ? networkProgress.error
                      ? t("setup.networkFailed")
                      : t("setup.networkDone")
                    : (networkProgress?.message ?? t("setup.applyingNetwork"))}
                </Alert>
              )}
              {finishError && <Alert tone="warn">{finishError}</Alert>}

              <Button disabled={finishing} onClick={() => void finish()} className="w-full">
                {finishing ? t("common.loading") : t("setup.finish")}
              </Button>
            </div>
          )}

          {step !== "admin" && step !== "review" && (
            <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--color-line)] pt-4">
              {canGoBack ? (
                <Button variant="ghost" onClick={goBack}>
                  {t("launch.back")}
                </Button>
              ) : (
                <span />
              )}
              <Button onClick={goNext}>{t("launch.next")}</Button>
            </div>
          )}
          {step === "review" && canGoBack && (
            <div className="mt-5 flex items-center justify-start border-t border-[var(--color-line)] pt-4">
              <Button variant="ghost" disabled={finishing} onClick={goBack}>
                {t("launch.back")}
              </Button>
            </div>
          )}
        </div>

        <div className="mt-5 flex justify-center">
          <LanguageSwitch />
        </div>
      </div>
    </div>
  );
}

function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
