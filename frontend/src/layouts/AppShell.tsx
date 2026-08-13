import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../auth/AuthContext";
import { LanguageSwitch } from "../components/LanguageSwitch";
import { PageTransition } from "../components/PageTransition";
import { api } from "../api/client";
import { Button } from "../components/ui";
import { readSidebarCollapsed, storeSidebarCollapsed } from "./sidebarPrefs";
import {
  IconAccount,
  IconAudit,
  IconBrand,
  IconChevron,
  IconLogout,
  IconSessions,
  IconSettings,
  IconShield,
  IconUsers,
} from "../components/ui/icons";

function navClass(collapsed: boolean) {
  return ({ isActive }: { isActive: boolean }) =>
    ["nav-item", collapsed ? "justify-center px-0" : "", isActive ? "nav-item-active" : ""]
      .filter(Boolean)
      .join(" ");
}

function NavItem({
  to,
  icon,
  label,
  collapsed,
}: {
  to: string;
  icon: ReactNode;
  label: string;
  collapsed: boolean;
}) {
  return (
    <NavLink to={to} className={navClass(collapsed)} title={label}>
      {icon}
      {!collapsed && <span>{label}</span>}
    </NavLink>
  );
}

export function AppShell() {
  const { t } = useTranslation();
  const { user, logout, accessToken } = useAuth();
  const location = useLocation();
  const isAdmin = user?.role === "SUPER_ADMIN";
  const [tlsPending, setTlsPending] = useState(false);
  const [restartBusy, setRestartBusy] = useState(false);
  const [collapsed, setCollapsed] = useState(() => readSidebarCollapsed());
  const [instanceName, setInstanceName] = useState("");
  const brandName = instanceName || t("brand");

  function toggleCollapsed() {
    setCollapsed((prev) => {
      const next = !prev;
      storeSidebarCollapsed(next);
      return next;
    });
  }

  useEffect(() => {
    if (!accessToken) return;
    let cancelled = false;
    void api
      .getViewerFeatures(accessToken)
      .then((f) => {
        if (!cancelled && f.instanceName) setInstanceName(f.instanceName);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  useEffect(() => {
    document.title = brandName;
  }, [brandName]);

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
      "/domain-check": t("nav.domainCheck"),
      "/account": t("nav.account"),
      "/admin/users": t("nav.users"),
      "/admin/sessions": t("nav.allSessions"),
      "/admin/settings": t("nav.settings"),
      "/admin/audit": t("nav.audit"),
    };
    if (location.pathname.startsWith("/sessions/") && location.pathname !== "/sessions") {
      return t("viewer.title");
    }
    return map[location.pathname] || brandName;
  }, [location.pathname, t, brandName]);

  return (
    <div className="relative flex min-h-full">
      <aside
        className={[
          "glass-panel sticky top-0 z-20 flex h-screen shrink-0 flex-col rounded-none border-y-0 border-l-0 transition-[width] duration-200",
          collapsed ? "w-[72px]" : "w-[var(--sidebar-w)]",
        ].join(" ")}
      >
        <div className={["flex items-center gap-2.5 py-5", collapsed ? "justify-center px-0" : "px-5"].join(" ")}>
          <div className="grid size-8 shrink-0 place-items-center rounded-[var(--radius-sm)] bg-[var(--color-signal)] text-white">
            <IconBrand size={16} />
          </div>
          {!collapsed && (
            <div className="min-w-0">
              <div className="truncate text-[13px] font-semibold tracking-tight text-[var(--color-snow)]">
                {brandName}
              </div>
              <div className="text-[11px] text-[var(--color-muted)]">{t("tagline")}</div>
            </div>
          )}
        </div>

        <nav className={["flex flex-1 flex-col gap-0.5 overflow-y-auto overflow-x-hidden", collapsed ? "px-2" : "px-3"].join(" ")}>
          {!collapsed && (
            <div className="mb-1 mt-2 px-3 text-[11px] font-medium text-[var(--color-muted)]">
              {t("nav.workspace")}
            </div>
          )}
          <NavItem to="/sessions" icon={<IconSessions size={17} />} label={t("nav.sessions")} collapsed={collapsed} />
          <NavItem to="/history" icon={<IconAudit size={17} />} label={t("nav.history")} collapsed={collapsed} />
          <NavItem
            to="/domain-check"
            icon={<IconShield size={17} />}
            label={t("nav.domainCheck")}
            collapsed={collapsed}
          />
          <NavItem to="/account" icon={<IconAccount size={17} />} label={t("nav.account")} collapsed={collapsed} />

          {isAdmin && (
            <>
              {!collapsed && (
                <div className="mb-1 mt-5 px-3 text-[11px] font-medium text-[var(--color-muted)]">
                  {t("nav.admin")}
                </div>
              )}
              <NavItem to="/admin/users" icon={<IconUsers size={17} />} label={t("nav.users")} collapsed={collapsed} />
              <NavItem
                to="/admin/sessions"
                icon={<IconSessions size={17} />}
                label={t("nav.allSessions")}
                collapsed={collapsed}
              />
              <NavItem
                to="/admin/settings"
                icon={<IconSettings size={17} />}
                label={t("nav.settings")}
                collapsed={collapsed}
              />
              <NavItem to="/admin/audit" icon={<IconAudit size={17} />} label={t("nav.audit")} collapsed={collapsed} />
            </>
          )}
        </nav>

        <div className={["mt-auto space-y-1 p-3", collapsed ? "px-2" : ""].join(" ")}>
          <div
            className={[
              "flex items-center rounded-[var(--radius-md)] py-2 hover:bg-[var(--color-surface-hover)]",
              collapsed ? "justify-center px-0" : "gap-2.5 px-2.5",
            ].join(" ")}
          >
            <div className="grid size-8 shrink-0 place-items-center rounded-full bg-[var(--color-panel-3)] text-xs font-semibold text-[var(--color-fog)]">
              {(user?.displayName || user?.email || "?").slice(0, 1).toUpperCase()}
            </div>
            {!collapsed && (
              <>
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
              </>
            )}
          </div>

          {collapsed && (
            <button
              type="button"
              onClick={() => void logout()}
              title={t("nav.logout")}
              className="btn-icon mx-auto flex"
            >
              <IconLogout size={15} />
            </button>
          )}

          <button
            type="button"
            onClick={toggleCollapsed}
            title={collapsed ? t("nav.expand") : t("nav.collapse")}
            className={[
              "nav-item w-full text-[var(--color-muted)]",
              collapsed ? "justify-center px-0" : "",
            ].join(" ")}
          >
            <IconChevron size={15} className={collapsed ? "" : "rotate-180"} />
            {!collapsed && <span>{t("nav.collapse")}</span>}
          </button>
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
