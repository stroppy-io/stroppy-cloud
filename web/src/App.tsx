import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { Layout } from "@/components/Layout";
import { Runs } from "@/pages/Runs";
import { NewRun } from "@/pages/NewRun";
import { RunDetail } from "@/pages/RunDetail";
import { SettingsPage } from "@/pages/Settings";
import { Presets } from "@/pages/Presets";
import { Monitoring } from "@/pages/Monitoring";
import { Login } from "@/pages/Login";
import { useAuth } from "@/hooks/useAuth";

function LoginRedirect() {
  const [params] = [new URLSearchParams(window.location.search)];
  const redirect = params.get("redirect") || "/";
  return <Navigate to={redirect} replace />;
}

export default function App() {
  const { isAuthenticated, user, login, logout } = useAuth();
  const location = useLocation();

  if (!isAuthenticated) {
    return (
      <Routes>
        <Route path="/login" element={<Login onLogin={login} />} />
        {/* Preserve original URL so we can redirect back after login */}
        <Route
          path="*"
          element={
            <Navigate to={`/login?redirect=${encodeURIComponent(location.pathname)}`} replace />
          }
        />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route element={<Layout user={user} onLogout={logout} />}>
        <Route path="/" element={<Runs />} />
        <Route path="/runs" element={<Runs />} />
        <Route path="/runs/new" element={<NewRun />} />
        <Route path="/runs/:id" element={<RunDetail />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/presets" element={<Presets />} />
        <Route path="/monitoring" element={<Monitoring />} />
      </Route>
      <Route path="/login" element={<LoginRedirect />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
