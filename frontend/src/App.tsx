import { Navigate, Route, Routes } from "react-router-dom";
import type { ReactNode } from "react";
import { useAuth } from "./auth/AuthContext";
import { AppShell } from "./layouts/AppShell";
import { AccountPage } from "./pages/AccountPage";
import { AdminAuditPage } from "./pages/AdminAuditPage";
import { AdminSessionsPage } from "./pages/AdminSessionsPage";
import { AdminSettingsPage } from "./pages/AdminSettingsPage";
import { AdminUsersPage } from "./pages/AdminUsersPage";
import { DomainCheckPage } from "./pages/DomainCheckPage";
import { LoginPage } from "./pages/LoginPage";
import { SetupPage } from "./pages/SetupPage";
import { SessionViewerPage } from "./pages/SessionViewerPage";
import { SessionsPage } from "./pages/SessionsPage";
import { HistoryPage } from "./pages/HistoryPage";
import { HistoryDetailPage } from "./pages/HistoryDetailPage";

function Protected({ children }: { children: ReactNode }) {
  const { user, bootstrapping } = useAuth();
  if (bootstrapping) {
    return (
      <div className="grid min-h-full place-items-center text-[var(--color-fog)]">
        Loading…
      </div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return children;
}

function AdminOnly({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  if (user?.role !== "SUPER_ADMIN") return <Navigate to="/sessions" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route
        path="/"
        element={
          <Protected>
            <AppShell />
          </Protected>
        }
      >
        <Route index element={<Navigate to="/sessions" replace />} />
        <Route path="sessions" element={<SessionsPage />} />
        <Route path="sessions/:id" element={<SessionViewerPage />} />
        <Route path="history" element={<HistoryPage />} />
        <Route path="history/:id" element={<HistoryDetailPage />} />
        <Route path="account" element={<AccountPage />} />
        <Route path="domain-check" element={<DomainCheckPage />} />
        <Route
          path="admin/users"
          element={
            <AdminOnly>
              <AdminUsersPage />
            </AdminOnly>
          }
        />
        <Route
          path="admin/sessions"
          element={
            <AdminOnly>
              <AdminSessionsPage />
            </AdminOnly>
          }
        />
        <Route
          path="admin/settings"
          element={
            <AdminOnly>
              <AdminSettingsPage />
            </AdminOnly>
          }
        />
        <Route
          path="admin/audit"
          element={
            <AdminOnly>
              <AdminAuditPage />
            </AdminOnly>
          }
        />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
