import { createBrowserRouter, Navigate } from "react-router"
import { useStore } from "@nanostores/react"
import { $isAuthenticated } from "@/stores/auth"
import { AppLayout } from "./layout"
import { LoginPage } from "@/pages/Login"
import { DashboardPage } from "@/pages/Dashboard"
import { EditorPage } from "@/pages/Editor"
import { RunsPage } from "@/pages/Runs"
import { RunDetailPage } from "@/pages/RunDetail"
import { SettingsPage } from "@/pages/SettingsPage"

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useStore($isAuthenticated)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    element: (
      <ProtectedRoute>
        <AppLayout />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: <DashboardPage /> },
      { path: "editor", element: <EditorPage /> },
      { path: "runs", element: <RunsPage /> },
      { path: "runs/:runId", element: <RunDetailPage /> },
      { path: "settings", element: <SettingsPage /> },
    ],
  },
])
