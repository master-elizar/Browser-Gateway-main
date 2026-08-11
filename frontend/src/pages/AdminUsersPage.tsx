import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api, type Role, type User } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Select,
} from "../components/ui";
import { DataTable } from "../components/ui/DataTable";
import { IconEmpty, IconPlus, IconUsers } from "../components/ui/icons";
import { useToast } from "../components/ui/Toast";

export function AdminUsersPage() {
  const { t } = useTranslation();
  const { accessToken, user: me } = useAuth();
  const toast = useToast();
  const [items, setItems] = useState<User[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState<Role>("USER");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await api.listUsers(accessToken);
      setItems(res.items);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (!accessToken) return;
    setBusy(true);
    try {
      await api.createUser(accessToken, { email, password, displayName, role });
      setEmail("");
      setPassword("");
      setDisplayName("");
      setRole("USER");
      await refresh();
      toast.push(t("admin.createUser"), "success");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "error";
      setError(msg);
      toast.push(msg, "danger");
    } finally {
      setBusy(false);
    }
  }

  async function toggleActive(u: User) {
    if (!accessToken) return;
    try {
      await api.patchUser(accessToken, u.id, { active: !(u.active ?? true) });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function setUserRole(u: User, next: Role) {
    if (!accessToken || u.role === next) return;
    try {
      await api.patchUser(accessToken, u.id, { role: next });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function removeUser(u: User) {
    if (!accessToken) return;
    if (!window.confirm(`Delete ${u.email}?`)) return;
    try {
      await api.deleteUser(accessToken, u.id);
      await refresh();
      toast.push(t("admin.delete"), "info");
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  return (
    <div className="animate-fade-in space-y-6">
      <PageHeader title={t("admin.usersTitle")} subtitle={t("admin.usersSubtitle")} />

      {error && <Alert tone="danger">{error}</Alert>}

      <Card>
        <CardHeader
          title={t("admin.createUser")}
          description={t("admin.usersSubtitle")}
          action={<span className="text-[var(--color-muted)]"><IconUsers size={18} /></span>}
        />
        <CardBody>
          <form onSubmit={onCreate} className="grid gap-4 lg:grid-cols-2">
            <Field label={t("login.email")}>
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t("login.emailPh")}
                required
                autoComplete="off"
              />
            </Field>
            <Field label={t("login.password")}>
              <Input
                type="password"
                minLength={8}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("login.passwordPh")}
                required
                autoComplete="new-password"
              />
            </Field>
            <Field label={t("login.displayName")}>
              <Input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder={t("login.displayNamePh")}
              />
            </Field>
            <Field label={t("admin.role")}>
              <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
                <option value="USER">USER</option>
                <option value="SUPER_ADMIN">SUPER_ADMIN</option>
              </Select>
            </Field>
            <div className="lg:col-span-2">
              <Button type="submit" disabled={busy}>
                <IconPlus size={16} />
                {t("admin.createUser")}
              </Button>
            </div>
          </form>
        </CardBody>
      </Card>

      <DataTable>
        <thead>
          <tr>
            <th>{t("login.email")}</th>
            <th>{t("login.displayName")}</th>
            <th>{t("admin.role")}</th>
            <th>{t("admin.active")}</th>
            <th>{t("admin.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr>
              <td colSpan={5} className="!p-0">
                <EmptyState icon={<IconEmpty />} title={t("admin.usersSubtitle")} />
              </td>
            </tr>
          ) : (
            items.map((u) => (
              <tr key={u.id}>
                <td className="font-medium">{u.email}</td>
                <td className="text-[var(--color-fog)]">{u.displayName || "—"}</td>
                <td>
                  <Select
                    className="!w-auto min-w-[9rem] py-1.5 font-mono text-xs"
                    value={u.role}
                    onChange={(e) => void setUserRole(u, e.target.value as Role)}
                  >
                    <option value="USER">USER</option>
                    <option value="SUPER_ADMIN">SUPER_ADMIN</option>
                  </Select>
                </td>
                <td>
                  <Badge tone={(u.active ?? true) ? "success" : "neutral"}>
                    {(u.active ?? true) ? t("admin.active") : t("admin.deactivate")}
                  </Badge>
                </td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="ghost"
                      className="!py-1.5 !text-xs"
                      onClick={() => void toggleActive(u)}
                      disabled={u.id === me?.id}
                    >
                      {(u.active ?? true) ? t("admin.deactivate") : t("admin.activate")}
                    </Button>
                    <Button
                      variant="danger"
                      className="!py-1.5 !text-xs"
                      onClick={() => void removeUser(u)}
                      disabled={u.id === me?.id}
                    >
                      {t("admin.delete")}
                    </Button>
                  </div>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </DataTable>
    </div>
  );
}
