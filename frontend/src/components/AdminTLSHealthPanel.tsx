import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { api, type TLSStatus } from "../api/client";
import { useAuth } from "../auth/AuthContext";

type InputMode = "file" | "text";
type Format = "pem" | "pkcs12";

export function AdminTLSPanel({
  onPendingChange,
}: {
  onPendingChange?: (pending: boolean) => void;
}) {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const [tls, setTls] = useState<TLSStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [inputMode, setInputMode] = useState<InputMode>("text");
  const [format, setFormat] = useState<Format>("pem");
  const [showChain, setShowChain] = useState(false);
  const [certPem, setCertPem] = useState("");
  const [keyPem, setKeyPem] = useState("");
  const [chainPem, setChainPem] = useState("");
  const [pkcs12B64, setPkcs12B64] = useState("");
  const [pkcs12Pass, setPkcs12Pass] = useState("");

  const refresh = useCallback(async () => {
    if (!accessToken) return;
    try {
      const tstat = await api.getTLS(accessToken);
      setTls(tstat);
      onPendingChange?.(tstat.pendingRestart);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }, [accessToken, onPendingChange]);

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), 15000);
    return () => window.clearInterval(id);
  }, [refresh]);

  async function readFileAsText(file: File) {
    return file.text();
  }

  async function readFileAsBase64(file: File) {
    const buf = await file.arrayBuffer();
    const bytes = new Uint8Array(buf);
    let binary = "";
    for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]!);
    return btoa(binary);
  }

  async function onSave(applyNow: boolean) {
    if (!accessToken) return;
    setBusy(true);
    setMsg(null);
    setError(null);
    try {
      const res = await api.putTLS(accessToken, {
        format,
        certificatePem: format === "pem" ? certPem : undefined,
        privateKeyPem: format === "pem" ? keyPem : undefined,
        chainPem: format === "pem" && showChain ? chainPem : undefined,
        pkcs12Base64: format === "pkcs12" ? pkcs12B64 : undefined,
        pkcs12Password: format === "pkcs12" ? pkcs12Pass : undefined,
        applyNow,
      });
      if (res.tls) setTls(res.tls);
      onPendingChange?.(res.pendingRestart);
      if (res.applyError) {
        setMsg(t("admin.tlsApplyFailed", { error: res.applyError }));
      } else if (res.pendingRestart) {
        setMsg(t("admin.tlsPending"));
      } else {
        setMsg(t("admin.tlsApplied"));
      }
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  async function onApply() {
    if (!accessToken) return;
    setBusy(true);
    setError(null);
    try {
      const res = await api.applyTLS(accessToken);
      if (res.tls) setTls(res.tls);
      onPendingChange?.(false);
      setMsg(t("admin.tlsApplied"));
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-lg border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-4 py-3 text-sm text-[var(--color-danger)]">
          {error}
        </div>
      )}
      {msg && (
        <div className="rounded-lg border border-[var(--color-signal)]/40 bg-[var(--color-signal)]/10 px-4 py-3 text-sm text-[var(--color-signal-2)]">
          {msg}
        </div>
      )}

      {tls && (
        <div className="rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-3 py-2 font-mono text-xs text-[var(--color-fog)]">
          <div>
            {tls.configured ? t("admin.tlsConfigured") : t("admin.tlsMissing")}
            {tls.pendingRestart ? ` · ${t("admin.tlsPendingShort")}` : ""}
          </div>
          {tls.subject && <div className="mt-1 break-all">{tls.subject}</div>}
          {tls.notAfter && (
            <div className="mt-1">
              {t("admin.tlsExpires")}: {tls.notAfter}
            </div>
          )}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        <Toggle
          left={t("admin.tlsModeFile")}
          right={t("admin.tlsModeText")}
          value={inputMode === "text"}
          onChange={(text) => setInputMode(text ? "text" : "file")}
        />
        <select
          className="rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-1.5 text-sm"
          value={format}
          onChange={(e) => setFormat(e.target.value as Format)}
        >
          <option value="pem">PEM / CRT+KEY</option>
          <option value="pkcs12">PKCS#12 (.p12/.pfx)</option>
        </select>
        {format === "pem" && (
          <button
            type="button"
            className="rounded-lg border border-[var(--color-line)] px-3 py-1.5 text-sm text-[var(--color-fog)] hover:text-[var(--color-snow)]"
            onClick={() => setShowChain((v) => !v)}
          >
            {showChain ? t("admin.tlsHideChain") : t("admin.tlsShowChain")}
          </button>
        )}
      </div>

        {format === "pem" && inputMode === "text" && (
          <div className="grid gap-3">
            <Field label={t("admin.tlsCert")}>
              <textarea
                className="input-field min-h-28 font-mono text-xs"
                value={certPem}
                onChange={(e) => setCertPem(e.target.value)}
                placeholder="-----BEGIN CERTIFICATE-----"
              />
            </Field>
            <Field label={t("admin.tlsKey")}>
              <textarea
                className="input-field min-h-28 font-mono text-xs"
                value={keyPem}
                onChange={(e) => setKeyPem(e.target.value)}
                placeholder="-----BEGIN PRIVATE KEY-----"
              />
            </Field>
            {showChain && (
              <Field label={t("admin.tlsChain")}>
                <textarea
                  className="input-field min-h-24 font-mono text-xs"
                  value={chainPem}
                  onChange={(e) => setChainPem(e.target.value)}
                  placeholder="-----BEGIN CERTIFICATE-----"
                />
              </Field>
            )}
          </div>
        )}

        {format === "pem" && inputMode === "file" && (
          <div className="grid w-full max-w-3xl gap-4">
            <Field label={t("admin.tlsCertFile")}>
              <input
                type="file"
                className="block w-full max-w-full text-sm text-[var(--color-fog)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-panel-2)] file:px-3 file:py-1.5 file:text-[var(--color-snow)]"
                accept=".pem,.crt,.cer,.cert"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) void readFileAsText(f).then(setCertPem);
                }}
              />
              {certPem && <p className="mt-1 text-xs text-[var(--color-signal-2)]">{t("admin.tlsFileLoaded")}</p>}
            </Field>
            <Field label={t("admin.tlsKeyFile")}>
              <input
                type="file"
                className="block w-full max-w-full text-sm text-[var(--color-fog)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-panel-2)] file:px-3 file:py-1.5 file:text-[var(--color-snow)]"
                accept=".pem,.key"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) void readFileAsText(f).then(setKeyPem);
                }}
              />
              {keyPem && <p className="mt-1 text-xs text-[var(--color-signal-2)]">{t("admin.tlsFileLoaded")}</p>}
            </Field>
            {showChain && (
              <Field label={t("admin.tlsChainFile")}>
                <input
                  type="file"
                  className="block w-full max-w-full text-sm text-[var(--color-fog)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-panel-2)] file:px-3 file:py-1.5 file:text-[var(--color-snow)]"
                  accept=".pem,.crt,.cer"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void readFileAsText(f).then(setChainPem);
                  }}
                />
              </Field>
            )}
          </div>
        )}

        {format === "pkcs12" && (
          <div className="grid w-full max-w-3xl gap-4">
            {inputMode === "file" ? (
              <Field label={t("admin.tlsPkcs12File")}>
                <input
                  type="file"
                  className="block w-full max-w-full text-sm text-[var(--color-fog)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-panel-2)] file:px-3 file:py-1.5 file:text-[var(--color-snow)]"
                  accept=".p12,.pfx"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void readFileAsBase64(f).then(setPkcs12B64);
                  }}
                />
              </Field>
            ) : (
              <Field label={t("admin.tlsPkcs12B64")}>
                <textarea
                  className="input-field min-h-24 font-mono text-xs"
                  value={pkcs12B64}
                  onChange={(e) => setPkcs12B64(e.target.value)}
                  placeholder="base64…"
                />
              </Field>
            )}
            <Field label={t("admin.tlsPkcs12Pass")}>
              <input
                type="password"
                className="input-field max-w-md"
                value={pkcs12Pass}
                onChange={(e) => setPkcs12Pass(e.target.value)}
                placeholder="••••••••"
                autoComplete="new-password"
              />
            </Field>
          </div>
        )}

        <div className="flex flex-wrap items-center gap-2 pt-1">
          <button
            type="button"
            disabled={busy}
            onClick={() => void onSave(true)}
            className="btn-primary shrink-0"
          >
            {t("admin.tlsSaveApply")}
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => void onSave(false)}
            className="btn-ghost shrink-0"
          >
            {t("admin.tlsSaveLater")}
          </button>
          {tls?.pendingRestart && (
            <button
              type="button"
              disabled={busy}
              onClick={() => void onApply()}
              className="shrink-0 rounded-lg border border-[var(--color-warn)]/50 px-4 py-2 text-sm text-[var(--color-warn)] hover:bg-[var(--color-warn)]/10 disabled:opacity-60"
            >
              {t("admin.tlsRestartNow")}
            </button>
          )}
        </div>
    </div>
  );
}

function Toggle({
  left,
  right,
  value,
  onChange,
}: {
  left: string;
  right: string;
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-line)] p-1 text-sm">
      <button
        type="button"
        className={`rounded-md px-3 py-1 ${!value ? "bg-[var(--color-panel-2)] text-[var(--color-snow)]" : "text-[var(--color-fog)]"}`}
        onClick={() => onChange(false)}
      >
        {left}
      </button>
      <button
        type="button"
        className={`rounded-md px-3 py-1 ${value ? "bg-[var(--color-panel-2)] text-[var(--color-snow)]" : "text-[var(--color-fog)]"}`}
        onClick={() => onChange(true)}
      >
        {right}
      </button>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-1 text-sm">
      <span className="text-[var(--color-fog)]">{label}</span>
      {children}
    </label>
  );
}
