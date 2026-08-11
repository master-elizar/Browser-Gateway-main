import { useEffect, useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { Alert, Button, Field, Input, Skeleton } from "../components/ui";
import { IconShield } from "../components/ui/icons";

export function SetupPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);
  const [setupKey, setSetupKey] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    void api
      .setupStatus()
      .then((s) => setNeedsSetup(s.needsSetup))
      .catch(() => setNeedsSetup(false));
  }, []);

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

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    setError(null);
    try {
      const pair = await api.setupComplete({ setupKey, email, password, displayName });
      localStorage.setItem("bg.accessToken", pair.accessToken);
      localStorage.setItem("bg.refreshToken", pair.refreshToken);
      window.location.href = "/sessions";
    } catch (err) {
      setError(err instanceof Error ? err.message : t("setup.error"));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="grid min-h-full place-items-center px-6 py-12">
      <div className="w-full max-w-[400px] animate-fade-in">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 grid size-14 place-items-center rounded-[var(--radius-lg)] bg-[var(--color-signal)] text-white shadow-[var(--shadow-md)]">
            <IconShield size={26} />
          </div>
          <h1 className="text-[1.75rem] font-semibold tracking-tight text-[var(--color-white)]">
            {t("brand")}
          </h1>
          <p className="mt-1 text-sm text-[var(--color-fog)]">{t("setup.subtitle")}</p>
        </div>

        <form onSubmit={onSubmit} className="ui-card space-y-3 p-6">
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
          {error && <Alert tone="warn">{error}</Alert>}
          <Button type="submit" disabled={pending} className="w-full">
            {pending ? t("common.loading") : t("setup.submit")}
          </Button>
        </form>

        <div className="mt-5 flex justify-center">
          <LanguageSwitch />
        </div>
      </div>
    </div>
  );
}
