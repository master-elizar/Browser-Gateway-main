import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuth } from "../auth/AuthContext";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { api } from "../api/client";
import { Alert, Button, Field, Input } from "../components/ui";
import { IconSessions } from "../components/ui/icons";

export function LoginPage() {
  const { t } = useTranslation();
  const { user, login, register } = useAuth();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [version, setVersion] = useState<string>("");

  useEffect(() => {
    void api
      .setupStatus()
      .then((s) => {
        if (s.needsSetup) window.location.replace("/setup");
      })
      .catch(() => undefined);
    void api
      .version()
      .then((v) => setVersion(`v${v.version} · stage ${v.stage}`))
      .catch(() => setVersion(""));
  }, []);

  if (user) return <Navigate to="/sessions" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    setError(null);
    try {
      if (mode === "register") {
        await register(email, password, displayName);
      } else {
        await login(email, password);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.error"));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="grid min-h-full place-items-center px-6 py-12">
      <div className="w-full max-w-[420px] animate-fade-in">
        <div className="mb-8 flex items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <div className="grid size-11 place-items-center rounded-[var(--radius-lg)] bg-[var(--color-signal-dim)] text-[var(--color-signal-2)] ring-1 ring-[var(--color-signal)]/25">
              <IconSessions size={22} />
            </div>
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-[var(--color-white)]">
                {t("brand")}
              </h1>
              <p className="mt-1 text-sm text-[var(--color-fog)]">{t("tagline")}</p>
              {version && (
                <p className="mt-2 font-mono text-[11px] text-[var(--color-signal-2)]">{version}</p>
              )}
            </div>
          </div>
          <LanguageSwitch />
        </div>

        <form onSubmit={onSubmit} className="glass-panel rounded-[var(--radius-xl)] p-6">
          <div className="mb-5 flex gap-1 rounded-[var(--radius-md)] bg-[var(--color-ink)]/70 p-1">
            <button
              type="button"
              className={tabClass(mode === "login")}
              onClick={() => setMode("login")}
            >
              {t("login.title")}
            </button>
            <button
              type="button"
              className={tabClass(mode === "register")}
              onClick={() => setMode("register")}
            >
              {t("login.register")}
            </button>
          </div>

          <div className="space-y-3">
            {mode === "register" && (
              <Field label={t("login.displayName")}>
                <Input
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder={t("login.displayNamePh")}
                />
              </Field>
            )}
            <Field label={t("login.email")}>
              <Input
                type="email"
                autoComplete="username"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t("login.emailPh")}
                required
              />
            </Field>
            <Field label={t("login.password")}>
              <Input
                type="password"
                autoComplete={mode === "register" ? "new-password" : "current-password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("login.passwordPh")}
                required
                minLength={8}
              />
            </Field>
          </div>

          {error && (
            <div className="mt-4">
              <Alert tone="warn">{error}</Alert>
            </div>
          )}

          <Button type="submit" disabled={pending} className="mt-5 w-full">
            {pending ? t("common.loading") : mode === "register" ? t("login.register") : t("login.submit")}
          </Button>
          <p className="mt-3 text-xs leading-relaxed text-[var(--color-muted)]">{t("login.hint")}</p>
        </form>
      </div>
    </div>
  );
}

function tabClass(active: boolean) {
  return [
    "flex-1 rounded-[var(--radius-sm)] px-3 py-2 text-sm transition",
    active
      ? "bg-[var(--color-panel-2)] font-medium text-[var(--color-snow)] shadow-[var(--shadow-sm)]"
      : "text-[var(--color-fog)] hover:text-[var(--color-snow)]",
  ].join(" ");
}
