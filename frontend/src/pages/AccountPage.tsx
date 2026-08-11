import { useTranslation } from "react-i18next";
import { ChangePasswordForm } from "../components/ChangePasswordForm";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { ThemePicker } from "../components/ThemePicker";
import { MotionToggle } from "../components/MotionToggle";
import { useAuth } from "../auth/AuthContext";
import { Badge, Card, CardBody, CardHeader, PageHeader } from "../components/ui";

export function AccountPage() {
  const { t } = useTranslation();
  const { user } = useAuth();

  return (
    <div className="mx-auto max-w-3xl animate-fade-in space-y-6">
      <PageHeader title={t("account.title")} subtitle={t("account.subtitle")} />

      <Card>
        <CardHeader title={t("account.profile")} description={t("account.subtitle")} />
        <CardBody>
          <dl className="divide-y divide-[var(--color-line)]">
            <div className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
              <dt className="text-sm text-[var(--color-fog)]">{t("login.email")}</dt>
              <dd className="text-sm font-medium text-[var(--color-snow)]">{user?.email}</dd>
            </div>
            <div className="flex items-center justify-between gap-4 py-3">
              <dt className="text-sm text-[var(--color-fog)]">{t("login.displayName")}</dt>
              <dd className="text-sm text-[var(--color-snow)]">{user?.displayName || "—"}</dd>
            </div>
            <div className="flex items-center justify-between gap-4 py-3">
              <dt className="text-sm text-[var(--color-fog)]">{t("admin.role")}</dt>
              <dd>
                <Badge tone="accent">{user?.role}</Badge>
              </dd>
            </div>
            <div className="flex items-center justify-between gap-4 py-3">
              <dt className="text-sm text-[var(--color-fog)]">{t("common.language")}</dt>
              <dd>
                <LanguageSwitch />
              </dd>
            </div>
          </dl>
        </CardBody>
      </Card>

      <Card>
        <CardHeader title={t("motion.title")} description={t("motion.subtitle")} />
        <CardBody>
          <MotionToggle />
        </CardBody>
      </Card>

      <Card>
        <CardHeader title={t("theme.title")} description={t("theme.subtitle")} />
        <CardBody>
          <ThemePicker />
        </CardBody>
      </Card>

      <Card>
        <CardHeader title={t("account.passwordSection")} description={t("account.passwordHint")} />
        <CardBody>
          <ChangePasswordForm />
        </CardBody>
      </Card>
    </div>
  );
}
