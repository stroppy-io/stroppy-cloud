import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { AuthProvider, useAuth } from "@/lib/auth"
import { AppLayout } from "@/components/AppLayout"
import { LoginPage } from "@/pages/LoginPage"
import { DashboardPage } from "@/pages/DashboardPage"
import { WorkloadsPage } from "@/pages/WorkloadsPage"
import { TopologyTemplatesPage } from "@/pages/TopologyTemplatesPage"
import { RunsPage } from "@/pages/RunsPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { NewTestPage } from "@/pages/NewTestPage"
import { TopologyEditorPage } from "@/pages/TopologyEditorPage"
import { SuiteDetailPage } from "@/pages/SuiteDetailPage"
import { Loader2 } from "lucide-react"

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AppRoutes() {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <LoginPage />} />
      <Route element={<AuthGuard><AppLayout /></AuthGuard>}>
        <Route index element={<DashboardPage />} />
        <Route path="workloads" element={<WorkloadsPage />} />
        <Route path="topologies" element={<TopologyTemplatesPage />} />
        <Route path="topologies/new" element={<TopologyEditorPage />} />
        <Route path="topologies/:id/edit" element={<TopologyEditorPage />} />
        <Route path="tests/new" element={<NewTestPage />} />
        <Route path="runs" element={<RunsPage />} />
        <Route path="suites/:id" element={<SuiteDetailPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}
