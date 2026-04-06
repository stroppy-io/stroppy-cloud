import { NavLink, Outlet } from "react-router-dom";
import {
  List,
  Play,
  GitCompare,
  Settings,
  Layers,
  Activity,
  LogOut,
  User,

  Users,
  KeyRound,
  Building2,
  ShieldCheck,
} from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { TenantSwitcher } from "@/components/TenantSwitcher";
import type { AuthUser } from "@/api/types";

const roleLevel: Record<string, number> = { viewer: 1, operator: 2, owner: 3 };

function userLevel(user: AuthUser): number {
  if (user.is_root) return 99;
  return roleLevel[user.role] || 0;
}

interface NavItem {
  to: string;
  icon: typeof List;
  label: string;
  minLevel: number; // 1=viewer, 2=operator, 3=owner, 99=root
}

const navItems: NavItem[] = [
  { to: "/", icon: List, label: "Runs", minLevel: 1 },
  { to: "/runs/new", icon: Play, label: "New Run", minLevel: 2 },
  { to: "/compare", icon: GitCompare, label: "Compare", minLevel: 1 },
  { to: "/presets", icon: Layers, label: "Presets", minLevel: 1 },
  { to: "/settings", icon: Settings, label: "Settings", minLevel: 1 },
  { to: "/members", icon: Users, label: "Members", minLevel: 3 },
  { to: "/tokens", icon: KeyRound, label: "API Tokens", minLevel: 3 },
  { to: "/admin/tenants", icon: Building2, label: "Admin (Tenants)", minLevel: 99 },
  { to: "/admin/users", icon: ShieldCheck, label: "Admin (Users)", minLevel: 99 },
];

export function Layout() {
  const { user, logout } = useAuth();
  const level = user ? userLevel(user) : 0;

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className="w-48 shrink-0 border-r border-border bg-[#080808] flex flex-col">
        <div className="p-4 border-b border-border space-y-2">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" />
            <span className="text-sm font-semibold tracking-tight">
              stroppy-cloud
            </span>
          </div>
          {user?.is_root && <TenantSwitcher />}
        </div>
        <nav className="flex-1 py-2 overflow-y-auto">
          {navItems
            .filter((item) => level >= item.minLevel)
            .map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  `flex items-center gap-2.5 px-4 py-2 text-sm transition-colors ${
                    isActive
                      ? "text-foreground bg-muted border-r-2 border-primary"
                      : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                  }`
                }
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            ))}
        </nav>
        <div className="border-t border-border">
          {user && (
            <div className="px-4 py-3 space-y-1">
              {user.tenant_name && (
                <div className="text-[10px] text-muted-foreground truncate">
                  {user.tenant_name}
                </div>
              )}
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <User className="h-3.5 w-3.5" />
                  <span className="truncate">{user.username}</span>
                </div>
                <button
                  onClick={logout}
                  title="Sign out"
                  className="text-muted-foreground hover:text-foreground transition-colors"
                >
                  <LogOut className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          )}
          <div className="px-4 py-3 border-t border-border">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <div className="w-2 h-2 bg-success" id="health-indicator" />
              <span id="health-text">Server</span>
            </div>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
