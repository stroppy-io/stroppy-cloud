import { useState } from "react"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Zap, Loader2, Database, Layers, Activity } from "lucide-react"

export function LoginPage() {
  const { login } = useAuth()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setLoading(true)
    try {
      await login(username, password)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed")
    }
    setLoading(false)
  }

  return (
    <div className="min-h-screen flex">
      {/* Left brand panel */}
      <div className="hidden lg:flex lg:w-[480px] xl:w-[560px] bg-card border-r border-border/60 flex-col justify-between p-10 relative overflow-hidden">
        <div className="absolute inset-0 pointer-events-none">
          <div className="absolute -top-40 -left-40 w-[500px] h-[500px] rounded-full bg-primary/5 blur-[100px]" />
          <div className="absolute -bottom-20 -right-20 w-[400px] h-[400px] rounded-full bg-chart-2/5 blur-[100px]" />
        </div>

        <div className="relative">
          <div className="flex items-center gap-3 mb-12">
            <div className="w-10 h-10 rounded-xl bg-primary flex items-center justify-center glow-primary">
              <Zap className="w-5 h-5 text-primary-foreground" />
            </div>
            <div>
              <div className="font-semibold text-lg tracking-tight">Stroppy Cloud</div>
              <div className="text-xs text-muted-foreground">Database Performance Testing</div>
            </div>
          </div>

          <h2 className="text-2xl font-semibold tracking-tight leading-snug mb-4">
            Automated benchmark<br />testing at scale
          </h2>
          <p className="text-sm text-muted-foreground leading-relaxed max-w-sm">
            Design database topologies, run performance workloads, and compare results across deployments.
          </p>
        </div>

        <div className="relative space-y-4">
          {[
            { icon: Layers, title: "Visual Topology Builder", desc: "Design PostgreSQL clusters with drag-and-drop" },
            { icon: Database, title: "Workload Library", desc: "Reusable benchmark scripts with auto-probing" },
            { icon: Activity, title: "Live Test Monitoring", desc: "Real-time status tracking and result comparison" },
          ].map(f => (
            <div key={f.title} className="flex items-start gap-3 p-3 rounded-xl bg-muted/30 border border-border/40">
              <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0 mt-0.5">
                <f.icon className="w-4 h-4 text-primary" />
              </div>
              <div>
                <div className="text-sm font-medium">{f.title}</div>
                <div className="text-xs text-muted-foreground">{f.desc}</div>
              </div>
            </div>
          ))}
        </div>

        <p className="relative text-[10px] text-muted-foreground/40 font-mono">v0.1.0</p>
      </div>

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center bg-background p-6">
        <div className="w-full max-w-[360px]">
          {/* Mobile logo */}
          <div className="flex items-center gap-2.5 mb-10 lg:hidden">
            <div className="w-9 h-9 rounded-xl bg-primary flex items-center justify-center">
              <Zap className="w-4.5 h-4.5 text-primary-foreground" />
            </div>
            <span className="font-semibold tracking-tight">Stroppy Cloud</span>
          </div>

          <div className="mb-8">
            <h1 className="text-xl font-semibold tracking-tight">Welcome back</h1>
            <p className="text-sm text-muted-foreground mt-1">Sign in to your account to continue</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <div className="p-3 text-[12px] text-destructive bg-destructive/10 border border-destructive/20 rounded-lg flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-destructive shrink-0" />
                {error}
              </div>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="username" className="text-xs font-medium">Username</Label>
              <Input id="username" value={username} onChange={e => setUsername(e.target.value)} autoFocus placeholder="admin" className="h-10" />
            </div>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="password" className="text-xs font-medium">Password</Label>
              </div>
              <Input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="Enter password" className="h-10" />
            </div>
            <Button type="submit" className="w-full h-10" disabled={loading}>
              {loading && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
              {loading ? "Signing in..." : "Sign in"}
            </Button>
          </form>
        </div>
      </div>
    </div>
  )
}
