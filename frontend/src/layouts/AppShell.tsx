import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../auth/AuthContext";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { PageTransition } from "../components/PageTransition";
import { api } from "../api/client";
import { Button } from "../components/ui";
import {
  IconAccount,
  IconAudit,
  IconLogout,
  IconSessions,
  IconSettings,
  IconUsers,
} from "../components/ui/icons";

function navClass({ isActive }: { isActive: boolean }) {
  return ["nav-item", isActive ? "nav-item-active" : ""].filter(Boolean).join(" ");
}

export function AppShell() {
  const { t } = useTranslation();
  const { user, logout, accessToken } = useAuth();
  const location = useLocation();
  const isAdmin = user?.role === "SUPER_ADMIN";
  const [tlsPending, setTlsPending] = useState(false);
  const [restartBusy, setRestartBusy] = useState(false);

  useEffect(() => {
    if (!isAdmin || !accessToken) return;
    let cancelled = false;
    const tick = async () => {
      try {
        const tls = await api.getTLS(accessToken);
        if (!cancelled) setTlsPending(tls.pendingRestart);
      } catch {
        /* ignore */
      }
    };
    void tick();
    const id = window.setInterval(() => void tick(), 30_000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [isAdmin, accessToken]);

  const crumb = useMemo(() => {
    const map: Record<string, string> = {
      "/sessions": t("nav.sessions"),
      "/account": t("nav.account"),
      "/admin/users": t("nav.users"),
      "/admin/sessions": t("nav.allSessions"),
      "/admin/settings": t("nav.settings"),
      "/admin/audit": t("nav.audit"),
    };
    if (location.pathname.startsWith("/sessions/") && location.pathname !== "/sessions") {
      return t("viewer.title");
    }
    return map[location.pathname] || t("brand");
  }, [location.pathname, t]);

  return (
    <div className="relative flex min-h-full">
      <aside
        className="glass-panel sticky top-0 z-20 flex h-screen w-[var(--sidebar-w)] shrink-0 flex-col border-r border-[var(--color-line)]"
        style={{ borderRadius: 0, borderTop: 0, borderBottom: 0, borderLeft: 0 }}
      >
        <div className="border-b border-[var(--color-line)] px-5 py-5">
          <div className="flex items-center gap-3">
            <div className="grid size-9 place-items-center rounded-[var(--radius-md)] bg-[var(--color-signal-dim)] text-[var(--color-signal-2)] ring-1 ring-[var(--color-signal)]/30">
              <IconSessions size={18} />
            </div>
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold tracking-tight text-[var(--color-white)]">
                {t("brand")}
              </div>
              <div className="truncate text-[11px] text-[var(--color-muted)]">{t("tagline")}</div>
            </div>
          </div>
        </div>

        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-4">
          <div className="mb-2 px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-muted)]">
            {t("nav.workspace")}
          </div>
          <NavLink to="/sessions" className={navClass} title={t("nav.sessions")}>
            <IconSessions size={17} />
            <span>{t("nav.sessions")}</span>
          </NavLink>
          <NavLink to="/history" className={navClass} title={t("nav.history")}>
            <IconAudit size={17} />
            <span>{t("nav.history")}</span>
          </NavLink>
          <NavLink to="/account" className={navClass} title={t("nav.account")}>
            <IconAccount size={17} />
            <span>{t("nav.account")}</span>
          </NavLink>

          {isAdmin && (
            <>
              <div className="mb-2 mt-5 px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-muted)]">
                {t("nav.admin")}
              </div>
              <NavLink to="/admin/users" className={navClass} title={t("nav.users")}>
                <IconUsers size={17} />
                <span>{t("nav.users")}</span>
              </NavLink>
              <NavLink to="/admin/sessions" className={navClass} title={t("nav.allSessions")}>
                <IconSessions size={17} />
                <span>{t("nav.allSessions")}</span>
              </NavLink>
              <NavLink to="/admin/settings" className={navClass} title={t("nav.settings")}>
                <IconSettings size={17} />
                <span>{t("nav.settings")}</span>
              </NavLink>
              <NavLink to="/admin/audit" className={navClass} title={t("nav.audit")}>
                <IconAudit size={17} />
                <span>{t("nav.audit")}</span>
              </NavLink>
            </>
          )}
        </nav>

        <div className="mt-auto border-t border-[var(--color-line)] p-3">
          <div className="mb-2 rounded-[var(--radius-md)] bg-[var(--color-panel-2)]/70 px-3 py-2.5">
            <div className="truncate text-sm font-medium text-[var(--color-snow)]">
              {user?.displayName || user?.email}
            </div>
            <div className="mt-0.5 truncate font-mono text-[10px] uppercase tracking-wider text-[var(--color-muted)]">
              {user?.role}
            </div>
          </div>
          <Button variant="ghost" className="w-full justify-start gap-2" onClick={() => void logout()}>
            <IconLogout size={16} />
            {t("nav.logout")}
          </Button>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {isAdmin && tlsPending && (
          <div className="alert alert-warn flex items-center justify-between gap-3 rounded-none border-x-0 border-t-0 px-6">
            <span>{t("admin.tlsBanner")}</span>
            <Button
              variant="ghost"
              disabled={restartBusy || !accessToken}
              onClick={async () => {
                if (!accessToken) return;
                setRestartBusy(true);
                try {
                  await api.applyTLS(accessToken);
                  setTlsPending(false);
                } finally {
                  setRestartBusy(false);
                }
              }}
            >
              {t("admin.tlsRestartNow")}
            </Button>
          </div>
        )}

        <header
          className="app-topbar sticky top-0 z-20 flex h-[var(--topbar-h)] items-center justify-between gap-4 border-b border-[var(--color-line)] bg-[var(--color-bg)]/75 px-6 backdrop-blur-xl"
        >
          <div className="flex min-w-0 items-center gap-2 text-sm">
            <span className="text-[var(--color-muted)]">{t("brand")}</span>
            <span className="text-[var(--color-line-strong)]">/</span>
            <span className="truncate font-medium text-[var(--color-snow)]">{crumb}</span>
          </div>
          <div className="flex items-center gap-3">
            <LanguageSwitch />
          </div>
        </header>

        <main className="relative z-10 flex-1 px-6 py-6">
          <div className="page-wrap">
            <PageTransition>
              <Outlet />
            </PageTransition>
          </div>
        </main>
      </div>
    </div>
  );
}
