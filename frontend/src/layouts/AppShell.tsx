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
  IconBrand,
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
      <aside className="glass-panel sticky top-0 z-20 flex h-screen w-[var(--sidebar-w)] shrink-0 flex-col rounded-none border-y-0 border-l-0">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <div className="grid size-8 place-items-center rounded-[var(--radius-sm)] bg-[var(--color-signal)] text-white">
            <IconBrand size={16} />
          </div>
          <div className="min-w-0">
            <div className="truncate text-[13px] font-semibold tracking-tight text-[var(--color-snow)]">
              {t("brand")}
            </div>
            <div className="text-[11px] text-[var(--color-muted)]">{t("tagline")}</div>
          </div>
        </div>

        <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto px-3">
          <div className="mb-1 mt-2 px-3 text-[11px] font-medium text-[var(--color-muted)]">
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
              <div className="mb-1 mt-5 px-3 text-[11px] font-medium text-[var(--color-muted)]">
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

        <div className="mt-auto p-3">
          <div className="mb-1 flex items-center gap-2.5 rounded-[var(--radius-md)] px-2.5 py-2 hover:bg-[var(--color-surface-hover)]">
            <div className="grid size-8 shrink-0 place-items-center rounded-full bg-[var(--color-panel-3)] text-xs font-semibold text-[var(--color-fog)]">
              {(user?.displayName || user?.email || "?").slice(0, 1).toUpperCase()}
            </div>
            <div className="min-w-0 flex-1">
              <div
                className="truncate text-[13px] font-medium text-[var(--color-snow)]"
                title={user?.displayName || user?.email}
              >
                {user?.displayName || user?.email}
              </div>
              <div className="truncate text-[11px] text-[var(--color-muted)]">{user?.role}</div>
            </div>
            <button
              type="button"
              onClick={() => void logout()}
              title={t("nav.logout")}
              className="btn-icon shrink-0"
            >
              <IconLogout size={15} />
            </button>
          </div>
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

        <header className="app-topbar sticky top-0 z-20 flex h-[var(--topbar-h)] items-center justify-between gap-4 border-b border-[var(--color-line)] bg-[var(--color-bg)]/70 px-6 backdrop-blur-xl">
          <div className="min-w-0 truncate text-[15px] font-semibold text-[var(--color-snow)]">{crumb}</div>
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
