import { Routes, Route, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { Layout } from "@/components/Layout";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { Runs } from "@/pages/Runs";
import { NewRun } from "@/pages/NewRun";
import { RunDetail } from "@/pages/RunDetail";
import { Compare } from "@/pages/Compare";
import { SettingsPage } from "@/pages/Settings";
import { Presets } from "@/pages/Presets";
import { Packages } from "@/pages/Packages";
import { PresetDesigner } from "@/pages/PresetDesigner";

import { Login } from "@/pages/Login";
import { SelectTenant } from "@/pages/SelectTenant";
import { AdminTenants } from "@/pages/AdminTenants";
import { AdminUsers } from "@/pages/AdminUsers";
import { TenantMembers } from "@/pages/TenantMembers";
import { TenantTokens } from "@/pages/TenantTokens";
import { AuthProvider } from "@/contexts/AuthContext";
import { useAuth } from "@/hooks/useAuth";

function AppRoutes() {
  const { isAuthenticated, isLoading, user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  const { selectTenant } = useAuth();

  // After login: auto-select tenant or redirect.
  useEffect(() => {
    if (!user || !isAuthenticated) return;
    // Skip if already on select-tenant or admin pages (root can access admin without tenant).
    if (location.pathname === "/select-tenant" || location.pathname.startsWith("/admin")) return;

    if (user.tenant_id) return; // tenant already selected

    const tenants = user.tenants || [];
    if (tenants.length === 1) {
      // Auto-select the only tenant.
      selectTenant(tenants[0].id);
    } else if (tenants.length > 1) {
      // Multiple tenants — show selector.
      navigate("/select-tenant", { replace: true });
    } else if (user.is_root) {
      // Root with no tenants — go to admin to create one.
      navigate("/admin/tenants", { replace: true });
    } else {
      // Non-root with no tenants — show selector (will display "no tenants" message).
      navigate("/select-tenant", { replace: true });
    }
  }, [user, isAuthenticated, selectTenant, navigate, location.pathname]);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Loading...
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="*"
          element={
            <Navigate
              to={`/login?redirect=${encodeURIComponent(location.pathname)}`}
              replace
            />
          }
        />
      </Routes>
    );
  }

  // No tenant selected yet — only show select-tenant and admin pages.
  if (!user?.tenant_id) {
    return (
      <Routes>
        <Route path="/select-tenant" element={<SelectTenant />} />
        <Route element={<Layout />}>
          <Route element={<ProtectedRoute requireRoot />}>
            <Route path="/admin/tenants" element={<AdminTenants />} />
            <Route path="/admin/users" element={<AdminUsers />} />
          </Route>
        </Route>
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={
          user?.is_root
            ? <Navigate to="/admin/tenants" replace />
            : <Navigate to="/select-tenant" replace />
        } />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route path="/select-tenant" element={<SelectTenant />} />

      <Route element={<Layout key={user?.tenant_id || ""} />}>
        {/* Everyone */}
        <Route path="/" element={<Runs />} />
        <Route path="/runs" element={<Runs />} />
        <Route path="/runs/:id" element={<RunDetail />} />
        <Route path="/compare" element={<Compare />} />
        <Route path="/packages" element={<Packages />} />
        <Route path="/presets" element={<Presets />} />

        <Route path="/settings" element={<SettingsPage />} />

        {/* Operator+ */}
        <Route element={<ProtectedRoute minRole="operator" />}>
          <Route path="/runs/new" element={<NewRun />} />
          <Route path="/presets/new" element={<PresetDesigner />} />
          <Route path="/presets/:id/edit" element={<PresetDesigner />} />
        </Route>

        {/* Owner+ */}
        <Route element={<ProtectedRoute minRole="owner" />}>
          <Route path="/members" element={<TenantMembers />} />
          <Route path="/tokens" element={<TenantTokens />} />
        </Route>

        {/* Root only */}
        <Route element={<ProtectedRoute requireRoot />}>
          <Route path="/admin/tenants" element={<AdminTenants />} />
          <Route path="/admin/users" element={<AdminUsers />} />
        </Route>
      </Route>

      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
