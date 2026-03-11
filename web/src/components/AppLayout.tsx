import { NavLink, Outlet, useLocation } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import {
  LayoutDashboard, Database, Layers, PlayCircle, Settings2, Zap, LogOut, User, ChevronRight
} from "lucide-react"

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/workloads", icon: Database, label: "Workloads" },
  { to: "/topologies", icon: Layers, label: "Topologies" },
  { to: "/runs", icon: PlayCircle, label: "Runs" },
  { to: "/settings", icon: Settings2, label: "Settings" },
]

export function AppLayout() {
  const { user, logout } = useAuth()
  const location = useLocation()

  const currentPage = navItems.find(item =>
    item.to === "/" ? location.pathname === "/" : location.pathname.startsWith(item.to)
  )

  return (
    <div className="h-screen flex bg-background">
      {/* Sidebar */}
      <aside className="w-[220px] border-r border-border/60 bg-sidebar flex flex-col shrink-0">
        {/* Logo */}
        <div className="h-14 px-4 flex items-center gap-2.5 border-b border-border/60">
          <div className="w-7 h-7 rounded-md bg-primary flex items-center justify-center">
            <Zap className="w-3.5 h-3.5 text-primary-foreground" />
          </div>
          <div className="flex flex-col">
            <span className="font-semibold text-[13px] leading-tight tracking-tight">Stroppy</span>
            <span className="text-[10px] text-muted-foreground leading-tight">Cloud Platform</span>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-2 py-3 space-y-0.5">
          <p className="px-3 mb-2 text-[10px] font-medium uppercase tracking-widest text-muted-foreground/60">Menu</p>
          {navItems.map(item => (
            <NavLink key={item.to} to={item.to} end={item.to === "/"} className={({ isActive }) =>
              cn(
                "group flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] transition-all duration-150",
                isActive
                  ? "bg-primary/10 text-primary font-medium"
                  : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
              )
            }>
              <item.icon className={cn(
                "w-4 h-4 transition-colors",
              )} />
              {item.label}
              <ChevronRight className="w-3 h-3 ml-auto opacity-0 -translate-x-1 group-hover:opacity-40 group-hover:translate-x-0 transition-all duration-150" />
            </NavLink>
          ))}
        </nav>

        {/* User section */}
        <div className="p-2 border-t border-border/60">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="w-full justify-start gap-2.5 px-3 h-10 hover:bg-muted/60">
                <div className="w-6 h-6 rounded-full bg-primary/10 border border-primary/20 flex items-center justify-center">
                  <User className="w-3 h-3 text-primary" />
                </div>
                <div className="flex flex-col items-start">
                  <span className="text-[12px] font-medium truncate">{user?.username ?? "User"}</span>
                  <span className="text-[10px] text-muted-foreground leading-tight">Admin</span>
                </div>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              <div className="px-2 py-1.5">
                <p className="text-xs font-medium">{user?.username}</p>
                <p className="text-[10px] text-muted-foreground">Administrator</p>
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout} className="text-destructive focus:text-destructive">
                <LogOut className="w-3.5 h-3.5 mr-2" />Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* Top bar */}
        <header className="h-14 border-b border-border/60 bg-background px-6 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2">
            {currentPage && (
              <>
                <currentPage.icon className="w-4 h-4 text-muted-foreground" />
                <span className="text-sm font-medium">{currentPage.label}</span>
              </>
            )}
          </div>
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-primary/10 border border-primary/20">
              <div className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
              <span className="text-[10px] font-mono text-primary">Connected</span>
            </div>
          </div>
        </header>

        {/* Page content */}
        <div className="flex-1 overflow-y-auto">
          <div className="p-6">
            <Outlet />
          </div>
        </div>
      </main>
    </div>
  )
}
