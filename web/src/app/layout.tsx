import { Outlet, NavLink, useLocation } from "react-router"
import { useStore } from "@nanostores/react"
import {
  LayoutDashboard,
  FlaskConical,
  Play,
  Settings,
  Sun,
  Moon,
  Database,
} from "lucide-react"
import { $theme, toggleTheme } from "@/stores/theme"
import { logout } from "@/stores/auth"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Separator } from "@/components/ui/separator"

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/editor", icon: FlaskConical, label: "Test Editor" },
  { to: "/runs", icon: Play, label: "Runs" },
  { to: "/settings", icon: Settings, label: "Settings" },
]

export function AppLayout() {
  const theme = useStore($theme)
  const location = useLocation()

  const currentPage = navItems.find((item) => {
    if (item.to === "/") return location.pathname === "/"
    return location.pathname.startsWith(item.to)
  })

  return (
    <TooltipProvider delayDuration={300}>
      <div className="flex h-screen w-screen flex-col overflow-hidden">
        {/* Toolbar */}
        <header className="flex h-9 shrink-0 items-center border-b bg-secondary/50 px-2">
          <div className="flex items-center gap-1.5 text-[13px] font-semibold">
            <Database className="h-4 w-4 text-primary" />
            <span>Stroppy</span>
          </div>

          <Separator orientation="vertical" className="mx-2 h-4" />

          <nav className="flex items-center gap-0.5 text-[12px] text-muted-foreground">
            {currentPage && (
              <span className="text-foreground">{currentPage.label}</span>
            )}
          </nav>

          <div className="ml-auto flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-6 w-6" onClick={toggleTheme}>
                  {theme === "dark" ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Toggle theme</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-[11px] text-muted-foreground"
                  onClick={logout}
                >
                  Logout
                </Button>
              </TooltipTrigger>
              <TooltipContent>Sign out</TooltipContent>
            </Tooltip>
          </div>
        </header>

        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <aside className="flex w-10 shrink-0 flex-col items-center border-r bg-sidebar py-2 gap-1">
            {navItems.map((item) => {
              const Icon = item.icon
              return (
                <Tooltip key={item.to}>
                  <TooltipTrigger asChild>
                    <NavLink
                      to={item.to}
                      end={item.to === "/"}
                      className={({ isActive }) =>
                        `flex h-8 w-8 items-center justify-center transition-colors ${
                          isActive
                            ? "bg-sidebar-accent text-foreground"
                            : "text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground"
                        }`
                      }
                    >
                      <Icon className="h-4 w-4" />
                    </NavLink>
                  </TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              )
            })}
          </aside>

          {/* Content */}
          <main className="flex-1 overflow-auto">
            <Outlet />
          </main>
        </div>

        {/* Status bar */}
        <footer className="flex h-6 shrink-0 items-center border-t bg-secondary/50 px-2 text-[11px] text-muted-foreground">
          <span>Stroppy Cloud</span>
          <span className="ml-auto font-mono">v0.1.0</span>
        </footer>
      </div>
    </TooltipProvider>
  )
}
