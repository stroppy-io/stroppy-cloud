import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Activity, Database, Layers, Plus, Clock, ArrowRight, Zap,
  PlayCircle, ArrowUpRight, Server
} from "lucide-react"
import type { Suite } from "@/proto/api/api_pb.ts"

const statusConfig: Record<number, { label: string; variant: "default" | "secondary" | "destructive" | "outline"; dot: string }> = {
  0: { label: "Unknown", variant: "outline", dot: "bg-muted-foreground" },
  1: { label: "Queued", variant: "secondary", dot: "bg-muted-foreground" },
  2: { label: "Running", variant: "default", dot: "bg-primary animate-pulse" },
  3: { label: "Completed", variant: "default", dot: "bg-emerald-500" },
  4: { label: "Failed", variant: "destructive", dot: "bg-destructive" },
  5: { label: "Cancelled", variant: "outline", dot: "bg-muted-foreground" },
}

function StatusBadge({ status }: { status: number }) {
  const s = statusConfig[status] ?? statusConfig[0]
  return (
    <Badge variant={s.variant} className="gap-1.5 font-mono text-[10px]">
      <span className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
      {s.label}
    </Badge>
  )
}

export function DashboardPage() {
  const [suites, setSuites] = useState<Suite[]>([])
  const [stats, setStats] = useState({ total: 0, workloads: 0, templates: 0 })

  useEffect(() => {
    api.listSuites({ limit: BigInt(6) }).then(res => {
      setSuites(res.suites)
      setStats(prev => ({ ...prev, total: Number(res.total) }))
    }).catch(() => {})
    api.listWorkloads({ limit: BigInt(1) }).then(res => {
      setStats(prev => ({ ...prev, workloads: Number(res.total) }))
    }).catch(() => {})
    api.listTopologyTemplates({ limit: BigInt(1) }).then(res => {
      setStats(prev => ({ ...prev, templates: Number(res.total) }))
    }).catch(() => {})
  }, [])

  const running = suites.filter(s => s.status === 2).length
  const completed = suites.filter(s => s.status === 3).length

  return (
    <div className="space-y-6">
      {/* Bento grid: top row */}
      <div className="grid grid-cols-12 gap-4">
        {/* Hero stat - large */}
        <Card className="col-span-5 relative overflow-hidden">
          <CardContent className="pt-6 pb-5">
            <div className="flex items-start justify-between mb-6">
              <div>
                <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Test Suites</p>
                <p className="text-4xl font-semibold mt-2 tracking-tight font-mono">{stats.total}</p>
              </div>
              <div className="w-11 h-11 rounded-xl bg-primary/10 flex items-center justify-center">
                <Activity className="w-5 h-5 text-primary" />
              </div>
            </div>
            <div className="flex gap-4 text-xs">
              <div className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-primary animate-pulse" />
                <span className="text-muted-foreground"><span className="font-mono font-medium text-foreground">{running}</span> running</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-emerald-500" />
                <span className="text-muted-foreground"><span className="font-mono font-medium text-foreground">{completed}</span> completed</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Small stats stacked */}
        <div className="col-span-3 grid grid-rows-2 gap-4">
          <Card>
            <CardContent className="pt-4 pb-3">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Workloads</p>
                  <p className="text-2xl font-semibold mt-1 font-mono">{stats.workloads}</p>
                </div>
                <div className="w-8 h-8 rounded-lg bg-chart-2/10 flex items-center justify-center">
                  <Database className="w-4 h-4 text-chart-2" />
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-4 pb-3">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Topologies</p>
                  <p className="text-2xl font-semibold mt-1 font-mono">{stats.templates}</p>
                </div>
                <div className="w-8 h-8 rounded-lg bg-chart-3/10 flex items-center justify-center">
                  <Layers className="w-4 h-4 text-chart-3" />
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Quick actions panel */}
        <Card className="col-span-4">
          <CardContent className="pt-5 pb-4">
            <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-4">Quick Actions</p>
            <div className="space-y-2">
              <Link to="/tests/new" className="group flex items-center gap-3 p-2.5 rounded-lg border border-dashed border-border hover:border-primary/40 hover:bg-primary/5 transition-all duration-150">
                <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                  <Zap className="w-4 h-4 text-primary" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium">New test suite</p>
                  <p className="text-[10px] text-muted-foreground truncate">Run benchmarks</p>
                </div>
                <ArrowUpRight className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </Link>
              <Link to="/topologies/new" className="group flex items-center gap-3 p-2.5 rounded-lg border border-dashed border-border hover:border-chart-3/40 hover:bg-chart-3/5 transition-all duration-150">
                <div className="w-8 h-8 rounded-lg bg-chart-3/10 flex items-center justify-center shrink-0">
                  <Server className="w-4 h-4 text-chart-3" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium">Design topology</p>
                  <p className="text-[10px] text-muted-foreground truncate">Visual cluster builder</p>
                </div>
                <ArrowUpRight className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </Link>
              <Link to="/workloads" className="group flex items-center gap-3 p-2.5 rounded-lg border border-dashed border-border hover:border-chart-2/40 hover:bg-chart-2/5 transition-all duration-150">
                <div className="w-8 h-8 rounded-lg bg-chart-2/10 flex items-center justify-center shrink-0">
                  <Database className="w-4 h-4 text-chart-2" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium">Add workload</p>
                  <p className="text-[10px] text-muted-foreground truncate">Upload benchmark scripts</p>
                </div>
                <ArrowUpRight className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Recent activity - full width feed */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-medium">Recent Activity</h2>
          <Button variant="ghost" size="sm" asChild className="text-muted-foreground text-xs h-7">
            <Link to="/runs">View all <ArrowRight className="w-3 h-3 ml-1" /></Link>
          </Button>
        </div>

        {suites.length === 0 ? (
          <Card>
            <CardContent className="py-12 flex flex-col items-center">
              <div className="w-12 h-12 rounded-2xl bg-muted flex items-center justify-center mb-3">
                <PlayCircle className="w-6 h-6 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium mb-1">No test runs yet</p>
              <p className="text-xs text-muted-foreground mb-4">Create your first test to get started</p>
              <Button size="sm" asChild>
                <Link to="/tests/new"><Plus className="w-3.5 h-3.5 mr-1.5" />New Test</Link>
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-2 lg:grid-cols-3 gap-3">
            {suites.map(s => (
              <Link key={s.suiteId} to={`/suites/${s.suiteId}`}
                className="group block p-4 rounded-xl border border-border/60 bg-card hover:border-primary/30 hover:shadow-md transition-all duration-200">
                <div className="flex items-center justify-between mb-3">
                  <code className="text-xs font-mono text-muted-foreground">{s.suiteId.slice(0, 8)}</code>
                  <StatusBadge status={s.status} />
                </div>
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-sm font-medium">{s.testSuite?.tests.length ?? 0} test(s)</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Clock className="w-3 h-3" />
                    {s.createdAt ? new Date(Number(s.createdAt.seconds) * 1000).toLocaleDateString() : "-"}
                  </div>
                  <ArrowRight className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
