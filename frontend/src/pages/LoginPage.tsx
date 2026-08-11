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
      .then((v) => setVersion(`v${v.version}`))
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
      <div className="w-full max-w-[400px] animate-fade-in">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 grid size-14 place-items-center rounded-[var(--radius-lg)] bg-[var(--color-signal)] text-white shadow-[var(--shadow-md)]">
            <IconSessions size={26} />
          </div>
          <h1 className="text-[1.75rem] font-semibold tracking-tight text-[var(--color-white)]">
            {t("brand")}
          </h1>
          <p className="mt-1 text-sm text-[var(--color-fog)]">{t("tagline")}</p>
          {version && <p className="mt-2 font-mono text-[11px] text-[var(--color-muted)]">{version}</p>}
        </div>

        <form onSubmit={onSubmit} className="ui-card space-y-4 p-6">
          <div className="flex gap-1 rounded-[var(--radius-md)] bg-[var(--color-panel-2)] p-1">
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

          {error && <Alert tone="warn">{error}</Alert>}

          <Button type="submit" disabled={pending} className="w-full">
            {pending ? t("common.loading") : mode === "register" ? t("login.register") : t("login.submit")}
          </Button>
          <p className="text-center text-xs leading-relaxed text-[var(--color-muted)]">{t("login.hint")}</p>
        </form>

        <div className="mt-5 flex justify-center">
          <LanguageSwitch />
        </div>
      </div>
    </div>
  );
}

function tabClass(active: boolean) {
  return [
    "flex-1 rounded-[calc(var(--radius-md)-4px)] px-3 py-1.5 text-sm font-medium transition",
    active
      ? "bg-[var(--color-panel)] text-[var(--color-snow)] shadow-[var(--shadow-sm)]"
      : "text-[var(--color-fog)] hover:text-[var(--color-snow)]",
  ].join(" ");
}
