import { NavLink, Outlet } from "react-router-dom";
import {
  List,
  Play,
  Settings,
  Layers,
  Activity,
  LogOut,
  User,
  MonitorCheck,
} from "lucide-react";

const navItems = [
  { to: "/", icon: List, label: "Runs" },
  { to: "/runs/new", icon: Play, label: "New Run" },
  { to: "/monitoring", icon: MonitorCheck, label: "Monitoring" },
  { to: "/presets", icon: Layers, label: "Presets" },
  { to: "/settings", icon: Settings, label: "Settings" },
];

interface LayoutProps {
  user: { username: string; role: string } | null;
  onLogout: () => void;
}

export function Layout({ user, onLogout }: LayoutProps) {
  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className="w-48 shrink-0 border-r border-border bg-[#080808] flex flex-col">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Activity className="h-4 w-4 text-primary" />
          <span className="text-sm font-semibold tracking-tight">
            stroppy-cloud
          </span>
        </div>
        <nav className="flex-1 py-2">
          {navItems.map((item) => (
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
            <div className="flex items-center justify-between px-4 py-3">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <User className="h-3.5 w-3.5" />
                <span>{user.username}</span>
              </div>
              <button
                onClick={onLogout}
                title="Sign out"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                <LogOut className="h-3.5 w-3.5" />
              </button>
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
