import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Alert, Button, Field, Input } from "./ui";
import { useToast } from "./ui/Toast";

export function ChangePasswordForm() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const toast = useToast();
  const [currentPassword, setCurrent] = useState("");
  const [newPassword, setNew] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!accessToken) return;
    if (newPassword !== confirm) {
      setError(t("account.passwordMismatch"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await api.changePassword(accessToken, currentPassword, newPassword);
      toast.push(t("account.passwordChanged"), "success");
      setCurrent("");
      setNew("");
      setConfirm("");
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("account.passwordError");
      setError(msg);
      toast.push(msg, "danger");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="max-w-md space-y-3">
      <Field label={t("account.currentPassword")}>
        <Input
          type="password"
          value={currentPassword}
          onChange={(e) => setCurrent(e.target.value)}
          placeholder={t("account.currentPasswordPh")}
          required
          autoComplete="current-password"
        />
      </Field>
      <Field label={t("account.newPassword")}>
        <Input
          type="password"
          value={newPassword}
          onChange={(e) => setNew(e.target.value)}
          placeholder={t("account.newPasswordPh")}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </Field>
      <Field label={t("account.confirmPassword")}>
        <Input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          placeholder={t("account.confirmPasswordPh")}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </Field>
      {error && <Alert tone="danger">{error}</Alert>}
      <Button type="submit" disabled={busy}>
        {busy ? t("common.loading") : t("account.changePassword")}
      </Button>
    </form>
  );
}
