import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

const levels: Record<string, number> = { viewer: 1, operator: 2, owner: 3 };

export function ProtectedRoute({
  minRole,
  requireRoot,
}: {
  minRole?: string;
  requireRoot?: boolean;
}) {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (requireRoot && !user.is_root) return <Navigate to="/" replace />;
  if (minRole) {
    if ((levels[user.role] || 0) < (levels[minRole] || 0) && !user.is_root) {
      return <Navigate to="/" replace />;
    }
  }
  return <Outlet />;
}
