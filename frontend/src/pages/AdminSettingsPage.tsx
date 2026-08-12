import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { api, type AppSettings } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { AdminHealthPanel } from "../components/AdminHealthPanel";
import { AdminTLSPanel } from "../components/AdminTLSHealthPanel";
import { AdminUpdatesPanel } from "../components/AdminUpdatesPanel";
import { ChangePasswordForm } from "../components/ChangePasswordForm";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { MotionToggle } from "../components/MotionToggle";
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardHeader,
  Field,
  Input,
  PageHeader,
  Select,
  Toggle,
} from "../components/ui";
import {
  IconActivity,
  IconGlobe,
  IconSettings,
  IconShield,
  IconSliders,
  IconSpark,
} from "../components/ui/icons";
import { useToast } from "../components/ui/Toast";

const empty: AppSettings = {
  maxConcurrentSessionsGlobal: 15,
  maxConcurrentSessionsPerUser: 3,
  idleTimeoutSec: 1800,
  maxSessionDurationSec: 14400,
  retentionBytes: 10 * 1024 * 1024 * 1024,
  logSessionLifecycle: true,
  logControlActions: true,
  logVisitedUrls: true,
  logDownloads: true,
  logNetworkDns: true,
  logNetworkHttp: true,
  logKeystrokes: false,
  allowRegistration: false,
  passwordMinLength: 8,
  passwordRequireComplexity: false,
  dnsMode: "docker",
  dnsServers: "8.8.8.8,1.1.1.1",
  dnsDohUrl: "https://cloudflare-dns.com/dns-query",
  tiEnabled: false,
  tiProvider: "multi",
  tiAutoEnrich: false,
  tiVirusTotalEnabled: false,
  tiApiKey: "",
  tiApiKeySet: false,
  tiUrlhausEnabled: false,
  tiThreatfoxEnabled: false,
  tiThreatfoxApiKey: "",
  tiThreatfoxApiKeySet: false,
  tiAbuseipdbEnabled: false,
  tiAbuseipdbApiKey: "",
  tiAbuseipdbApiKeySet: false,
  tiOtxEnabled: false,
  tiOtxApiKey: "",
  tiOtxApiKeySet: false,
  tiSpamhausEnabled: false,
  viewerWebrtcEnabled: true,
  viewerNovncEnabled: true,
  viewerFitEnabled: true,
  viewerStretchEnabled: true,
  viewerClipboardEnabled: true,
  viewerUploadEnabled: true,
  viewerDownloadsEnabled: true,
  viewerNetworkEnabled: true,
  historyRetentionDays: 30,
  downloadZipPasswordDefault: "",
  downloadZipPasswordDefaultSet: false,
};

function clearTIKeys(s: AppSettings): AppSettings {
  return {
    ...s,
    tiApiKey: "",
    tiThreatfoxApiKey: "",
    tiAbuseipdbApiKey: "",
    tiOtxApiKey: "",
    downloadZipPasswordDefault: "",
  };
}

type SectionId = "general" | "appearance" | "security" | "logging" | "integrations" | "advanced";

export function AdminSettingsPage() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const toast = useToast();
  const [settings, setSettings] = useState<AppSettings>(empty);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [section, setSection] = useState<SectionId>("general");

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const s = await api.getSettings(accessToken);
      setSettings(clearTIKeys(s));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!accessToken) return;
    setBusy(true);
    try {
      const s = await api.putSettings(accessToken, settings);
      setSettings(clearTIKeys(s));
      setError(null);
      toast.push(t("admin.saved"), "success");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "error";
      setError(msg);
      toast.push(msg, "danger");
    } finally {
      setBusy(false);
    }
  }

  function num(key: keyof AppSettings, value: string) {
    const n = Number(value);
    if (!Number.isFinite(n)) return;
    setSettings((s) => ({ ...s, [key]: n }));
  }

  function toggle(key: keyof AppSettings) {
    setSettings((s) => ({ ...s, [key]: !s[key] }));
  }

  const nav: { id: SectionId; label: string; icon: ReactNode }[] = [
    { id: "general", label: t("admin.settingsGeneral"), icon: <IconSliders size={16} /> },
    { id: "appearance", label: t("admin.settingsAppearance"), icon: <IconSpark size={16} /> },
    { id: "security", label: t("admin.settingsSecurity"), icon: <IconShield size={16} /> },
    { id: "logging", label: t("admin.settingsLogging"), icon: <IconActivity size={16} /> },
    { id: "integrations", label: t("admin.settingsIntegrations"), icon: <IconGlobe size={16} /> },
    { id: "advanced", label: t("admin.settingsAdvanced"), icon: <IconSettings size={16} /> },
  ];

  return (
    <div className="animate-fade-in space-y-6">
      <PageHeader title={t("admin.settingsTitle")} subtitle={t("admin.settingsSubtitle")} />

      {error && <Alert tone="danger">{error}</Alert>}

      <div className="grid gap-6 xl:grid-cols-[240px_minmax(0,1fr)]">
        <aside className="ui-card h-fit p-2 xl:sticky xl:top-[calc(var(--topbar-h)+1.5rem)]">
          <nav className="flex flex-col gap-0.5">
            {nav.map((item) => (
              <button
                key={item.id}
                type="button"
                className={[
                  "settings-nav-item flex items-center gap-2",
                  section === item.id ? "settings-nav-item-active" : "",
                ].join(" ")}
                onClick={() => setSection(item.id)}
              >
                <span className="text-[var(--color-muted)]">{item.icon}</span>
                {item.label}
              </button>
            ))}
          </nav>
        </aside>

        <div className="min-w-0 space-y-4">
          {section === "general" && (
            <form onSubmit={onSave} className="space-y-4">
              <Card>
                <CardHeader
                  title={t("admin.subLimitsSessions")}
                  description={t("admin.settingsGeneralDesc")}
                />
                <CardBody>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Field label={t("admin.maxGlobal")}>
                      <Input
                        type="number"
                        min={1}
                        value={settings.maxConcurrentSessionsGlobal}
                        onChange={(e) => num("maxConcurrentSessionsGlobal", e.target.value)}
                      />
                    </Field>
                    <Field label={t("admin.maxPerUser")}>
                      <Input
                        type="number"
                        min={1}
                        value={settings.maxConcurrentSessionsPerUser}
                        onChange={(e) => num("maxConcurrentSessionsPerUser", e.target.value)}
                      />
                    </Field>
                    <Field label={t("admin.idleTimeout")}>
                      <Input
                        type="number"
                        min={60}
                        value={settings.idleTimeoutSec}
                        onChange={(e) => num("idleTimeoutSec", e.target.value)}
                      />
                    </Field>
                    <Field label={t("admin.maxDuration")}>
                      <Input
                        type="number"
                        min={60}
                        value={settings.maxSessionDurationSec}
                        onChange={(e) => num("maxSessionDurationSec", e.target.value)}
                      />
                    </Field>
                  </div>
                </CardBody>
              </Card>
              <Card>
                <CardHeader title={t("admin.subLimitsRetention")} />
                <CardBody>
                  <Field label={t("admin.retentionGb")}>
                    <Input
                      type="number"
                      min={1}
                      step={0.5}
                      className="max-w-xs"
                      value={Math.round((settings.retentionBytes / (1024 * 1024 * 1024)) * 10) / 10}
                      onChange={(e) => {
                        const gb = Number(e.target.value);
                        if (!Number.isFinite(gb) || gb <= 0) return;
                        setSettings((s) => ({
                          ...s,
                          retentionBytes: Math.round(gb * 1024 * 1024 * 1024),
                        }));
                      }}
                    />
                  </Field>
                  <Field label={t("history.retentionDays")} hint={t("history.retentionDaysHint")}>
                    <Input
                      type="number"
                      min={0}
                      className="max-w-xs"
                      value={settings.historyRetentionDays ?? 30}
                      onChange={(e) => num("historyRetentionDays", e.target.value)}
                    />
                  </Field>
                </CardBody>
              </Card>
              <Button type="submit" disabled={busy}>
                {t("common.save")}
              </Button>
            </form>
          )}

          {section === "appearance" && (
            <div className="space-y-4">
              <Card>
                <CardHeader
                  title={t("admin.settingsAppearance")}
                  description={t("admin.settingsAppearanceDesc")}
                />
                <CardBody>
                  <div className="flex items-center justify-between gap-4 rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-4 py-3">
                    <div>
                      <div className="text-sm text-[var(--color-snow)]">{t("common.language")}</div>
                      <div className="text-xs text-[var(--color-muted)]">{t("admin.settingsLanguageHint")}</div>
                    </div>
                    <LanguageSwitch />
                  </div>
                </CardBody>
              </Card>
              <Card>
                <CardHeader title={t("motion.title")} description={t("motion.subtitle")} />
                <CardBody>
                  <MotionToggle />
                </CardBody>
              </Card>
              <form onSubmit={onSave} className="space-y-4">
                <Card>
                  <CardHeader title={t("admin.viewerTitle")} description={t("admin.viewerHint")} />
                  <CardBody className="space-y-2">
                    {(
                      [
                        ["viewerWebrtcEnabled", "admin.viewerWebrtc", "admin.viewerWebrtcHint"],
                        ["viewerNovncEnabled", "admin.viewerNovnc", "admin.viewerNovncHint"],
                        ["viewerFitEnabled", "admin.viewerFit", "admin.viewerFitHint"],
                        ["viewerStretchEnabled", "admin.viewerStretch", "admin.viewerStretchHint"],
                        ["viewerClipboardEnabled", "admin.viewerClipboard", "admin.viewerClipboardHint"],
                        ["viewerUploadEnabled", "admin.viewerUpload", "admin.viewerUploadHint"],
                        ["viewerDownloadsEnabled", "admin.viewerDownloads", "admin.viewerDownloadsHint"],
                        ["viewerNetworkEnabled", "admin.viewerNetwork", "admin.viewerNetworkHint"],
                      ] as const
                    ).map(([key, label, hint]) => (
                      <Toggle
                        key={key}
                        checked={Boolean(settings[key])}
                        onChange={() => setSettings((s) => ({ ...s, [key]: !s[key] }))}
                        label={t(label)}
                        description={t(hint)}
                      />
                    ))}
                  </CardBody>
                </Card>
                <Button type="submit" disabled={busy}>
                  {t("common.save")}
                </Button>
              </form>
            </div>
          )}

          {section === "security" && (
            <div className="space-y-4">
              <Card>
                <CardHeader title={t("account.passwordSection")} description={t("account.passwordHint")} />
                <CardBody>
                  <ChangePasswordForm />
                </CardBody>
              </Card>
              <form onSubmit={onSave} className="space-y-4">
                <Card>
                  <CardHeader title={t("admin.subLimitsPassword")} />
                  <CardBody className="space-y-2">
                    <Field label={t("admin.passwordMinLength")}>
                      <Input
                        type="number"
                        min={6}
                        max={128}
                        className="max-w-xs"
                        value={settings.passwordMinLength || 8}
                        onChange={(e) => num("passwordMinLength", e.target.value)}
                      />
                    </Field>
                    <Toggle
                      checked={Boolean(settings.passwordRequireComplexity)}
                      onChange={() => toggle("passwordRequireComplexity")}
                      label={t("admin.passwordComplexity")}
                    />
                    <Toggle
                      checked={Boolean(settings.allowRegistration)}
                      onChange={() => toggle("allowRegistration")}
                      label={t("admin.allowRegistration")}
                    />
                    <Field label={t("history.zipPassword")} hint={t("history.zipPasswordHint")}>
                      <Input
                        type="password"
                        autoComplete="new-password"
                        placeholder={
                          settings.downloadZipPasswordDefaultSet
                            ? "••••••••"
                            : t("history.zipPasswordPh")
                        }
                        value={settings.downloadZipPasswordDefault || ""}
                        onChange={(e) =>
                          setSettings((s) => ({ ...s, downloadZipPasswordDefault: e.target.value }))
                        }
                      />
                    </Field>
                  </CardBody>
                </Card>
                <Button type="submit" disabled={busy}>
                  {t("common.save")}
                </Button>
              </form>
              <Card>
                <CardHeader title={t("admin.tlsTitle")} description={t("admin.tlsHint")} />
                <CardBody>
                  <AdminTLSPanel />
                </CardBody>
              </Card>
            </div>
          )}

          {section === "logging" && (
            <form onSubmit={onSave} className="space-y-4">
              <Card>
                <CardHeader
                  title={t("admin.subLoggingSession")}
                  description={t("admin.settingsLoggingDesc")}
                />
                <CardBody className="space-y-1">
                  <Toggle
                    checked={Boolean(settings.logSessionLifecycle)}
                    onChange={() => toggle("logSessionLifecycle")}
                    label={t("admin.logLifecycle")}
                  />
                  <Toggle
                    checked={Boolean(settings.logControlActions)}
                    onChange={() => toggle("logControlActions")}
                    label={t("admin.logControls")}
                  />
                  <Toggle
                    checked={Boolean(settings.logVisitedUrls)}
                    onChange={() => toggle("logVisitedUrls")}
                    label={t("admin.logUrls")}
                  />
                  <Toggle
                    checked={Boolean(settings.logDownloads)}
                    onChange={() => toggle("logDownloads")}
                    label={t("admin.logDownloads")}
                  />
                  <Toggle
                    checked={Boolean(settings.logKeystrokes)}
                    onChange={() => toggle("logKeystrokes")}
                    label={t("admin.logKeys")}
                  />
                </CardBody>
              </Card>
              <Card>
                <CardHeader title={t("admin.subLoggingNetwork")} />
                <CardBody className="space-y-1">
                  <Toggle
                    checked={Boolean(settings.logNetworkDns)}
                    onChange={() => toggle("logNetworkDns")}
                    label={t("admin.logDns")}
                  />
                  <Toggle
                    checked={Boolean(settings.logNetworkHttp)}
                    onChange={() => toggle("logNetworkHttp")}
                    label={t("admin.logHttp")}
                  />
                </CardBody>
              </Card>
              <Button type="submit" disabled={busy}>
                {t("common.save")}
              </Button>
            </form>
          )}

          {section === "integrations" && (
            <div className="space-y-4">
              <Card>
                <CardHeader title={t("admin.healthTitle")} description={t("admin.healthHint")} />
                <CardBody>
                  <AdminHealthPanel />
                </CardBody>
              </Card>
              <form onSubmit={onSave} className="space-y-4">
                <Card>
                  <CardHeader title={t("admin.dnsTitle")} description={t("admin.dnsHint")} />
                  <CardBody>
                    <div className="min-w-0 space-y-4">
                      <Field label={t("admin.dnsMode")} hint={t("admin.dnsHint")}>
                        <Select
                          value={settings.dnsMode || "docker"}
                          onChange={(e) => setSettings((s) => ({ ...s, dnsMode: e.target.value }))}
                        >
                          <option value="docker">{t("admin.dnsModeDocker")}</option>
                          <option value="custom">{t("admin.dnsModeCustom")}</option>
                          <option value="doh">{t("admin.dnsModeDoh")}</option>
                          <option value="custom_doh">{t("admin.dnsModeCustomDoh")}</option>
                        </Select>
                      </Field>

                      <Field
                        label={t("admin.dnsServers")}
                        hint={
                          settings.dnsMode === "custom" || settings.dnsMode === "custom_doh"
                            ? t("admin.dnsServersHintActive")
                            : t("admin.dnsServersHintIdle")
                        }
                      >
                        <Input
                          className="min-w-0 w-full font-mono text-sm"
                          placeholder="8.8.8.8,1.1.1.1"
                          value={settings.dnsServers || ""}
                          onChange={(e) => setSettings((s) => ({ ...s, dnsServers: e.target.value }))}
                          spellCheck={false}
                          autoComplete="off"
                        />
                      </Field>

                      <Field
                        label={t("admin.dnsDohUrl")}
                        hint={
                          settings.dnsMode === "doh" || settings.dnsMode === "custom_doh"
                            ? t("admin.dnsDohHintActive")
                            : t("admin.dnsDohHintIdle")
                        }
                      >
                        <Input
                          className="min-w-0 w-full font-mono text-sm"
                          type="text"
                          inputMode="url"
                          placeholder="https://cloudflare-dns.com/dns-query"
                          value={settings.dnsDohUrl || ""}
                          onChange={(e) => setSettings((s) => ({ ...s, dnsDohUrl: e.target.value }))}
                          spellCheck={false}
                          autoComplete="off"
                        />
                      </Field>
                    </div>
                  </CardBody>
                </Card>
                <Card>
                  <CardHeader title={t("admin.tiTitle")} description={t("admin.tiHint")} />
                  <CardBody className="space-y-5">
                    <Toggle
                      checked={Boolean(settings.tiEnabled)}
                      onChange={() => setSettings((s) => ({ ...s, tiEnabled: !s.tiEnabled }))}
                      label={t("admin.tiEnabled")}
                      description={t("admin.tiEnabledHint")}
                    />
                    <Toggle
                      checked={Boolean(settings.tiAutoEnrich)}
                      onChange={() => setSettings((s) => ({ ...s, tiAutoEnrich: !s.tiAutoEnrich }))}
                      label={t("admin.tiAutoEnrich")}
                      description={t("admin.tiAutoEnrichHint")}
                    />

                    <div className="space-y-3 border-t border-[var(--color-line)]/60 pt-4">
                      <p className="text-xs text-[var(--color-fog)]">{t("admin.tiProvidersHint")}</p>

                      <Toggle
                        checked={Boolean(settings.tiVirusTotalEnabled)}
                        onChange={() =>
                          setSettings((s) => ({ ...s, tiVirusTotalEnabled: !s.tiVirusTotalEnabled }))
                        }
                        label={t("admin.tiVT")}
                        description={t("admin.tiVTHint")}
                      />
                      <Field
                        label={t("admin.tiApiKey")}
                        hint={settings.tiApiKeySet ? t("admin.tiApiKeySetHint") : t("admin.tiApiKeyHint")}
                      >
                        <Input
                          className="font-mono text-sm"
                          type="password"
                          autoComplete="off"
                          placeholder={settings.tiApiKeySet ? "••••••••" : "vt-…"}
                          value={settings.tiApiKey || ""}
                          onChange={(e) => setSettings((s) => ({ ...s, tiApiKey: e.target.value }))}
                          spellCheck={false}
                        />
                      </Field>

                      <Toggle
                        checked={Boolean(settings.tiUrlhausEnabled)}
                        onChange={() =>
                          setSettings((s) => ({ ...s, tiUrlhausEnabled: !s.tiUrlhausEnabled }))
                        }
                        label={t("admin.tiUrlhaus")}
                        description={t("admin.tiUrlhausHint")}
                      />

                      <Toggle
                        checked={Boolean(settings.tiThreatfoxEnabled)}
                        onChange={() =>
                          setSettings((s) => ({ ...s, tiThreatfoxEnabled: !s.tiThreatfoxEnabled }))
                        }
                        label={t("admin.tiThreatfox")}
                        description={t("admin.tiThreatfoxHint")}
                      />
                      <Field
                        label={t("admin.tiThreatfoxKey")}
                        hint={
                          settings.tiThreatfoxApiKeySet
                            ? t("admin.tiApiKeySetHint")
                            : t("admin.tiThreatfoxKeyHint")
                        }
                      >
                        <Input
                          className="font-mono text-sm"
                          type="password"
                          autoComplete="off"
                          placeholder={settings.tiThreatfoxApiKeySet ? "••••••••" : ""}
                          value={settings.tiThreatfoxApiKey || ""}
                          onChange={(e) =>
                            setSettings((s) => ({ ...s, tiThreatfoxApiKey: e.target.value }))
                          }
                          spellCheck={false}
                        />
                      </Field>

                      <Toggle
                        checked={Boolean(settings.tiAbuseipdbEnabled)}
                        onChange={() =>
                          setSettings((s) => ({ ...s, tiAbuseipdbEnabled: !s.tiAbuseipdbEnabled }))
                        }
                        label={t("admin.tiAbuseipdb")}
                        description={t("admin.tiAbuseipdbHint")}
                      />
                      <Field
                        label={t("admin.tiAbuseipdbKey")}
                        hint={
                          settings.tiAbuseipdbApiKeySet
                            ? t("admin.tiApiKeySetHint")
                            : t("admin.tiAbuseipdbKeyHint")
                        }
                      >
                        <Input
                          className="font-mono text-sm"
                          type="password"
                          autoComplete="off"
                          placeholder={settings.tiAbuseipdbApiKeySet ? "••••••••" : ""}
                          value={settings.tiAbuseipdbApiKey || ""}
                          onChange={(e) =>
                            setSettings((s) => ({ ...s, tiAbuseipdbApiKey: e.target.value }))
                          }
                          spellCheck={false}
                        />
                      </Field>

                      <Toggle
                        checked={Boolean(settings.tiOtxEnabled)}
                        onChange={() => setSettings((s) => ({ ...s, tiOtxEnabled: !s.tiOtxEnabled }))}
                        label={t("admin.tiOtx")}
                        description={t("admin.tiOtxHint")}
                      />
                      <Field
                        label={t("admin.tiOtxKey")}
                        hint={settings.tiOtxApiKeySet ? t("admin.tiApiKeySetHint") : t("admin.tiOtxKeyHint")}
                      >
                        <Input
                          className="font-mono text-sm"
                          type="password"
                          autoComplete="off"
                          placeholder={settings.tiOtxApiKeySet ? "••••••••" : ""}
                          value={settings.tiOtxApiKey || ""}
                          onChange={(e) => setSettings((s) => ({ ...s, tiOtxApiKey: e.target.value }))}
                          spellCheck={false}
                        />
                      </Field>

                      <Toggle
                        checked={Boolean(settings.tiSpamhausEnabled)}
                        onChange={() =>
                          setSettings((s) => ({ ...s, tiSpamhausEnabled: !s.tiSpamhausEnabled }))
                        }
                        label={t("admin.tiSpamhaus")}
                        description={t("admin.tiSpamhausHint")}
                      />
                    </div>
                  </CardBody>
                </Card>
                <Button type="submit" disabled={busy}>
                  {t("common.save")}
                </Button>
              </form>
            </div>
          )}

          {section === "advanced" && (
            <div className="space-y-4">
              <Card>
                <CardHeader title={t("admin.updatesTitle")} description={t("admin.updatesHint")} />
                <CardBody>
                  <AdminUpdatesPanel />
                </CardBody>
              </Card>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
