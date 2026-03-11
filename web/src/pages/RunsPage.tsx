import { useEffect, useState, useCallback } from "react"
import { Link } from "react-router-dom"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  PlayCircle, Clock, ArrowRight, Activity, CheckCircle2, XCircle, Timer, Plus
} from "lucide-react"
import type { Suite } from "@/proto/api/api_pb.ts"

const statusConfig: Record<number, { label: string; variant: "default" | "secondary" | "destructive" | "outline"; dot: string; icon: typeof Activity }> = {
  0: { label: "Unknown", variant: "outline", dot: "bg-muted-foreground", icon: Activity },
  1: { label: "Queued", variant: "secondary", dot: "bg-muted-foreground", icon: Timer },
  2: { label: "Running", variant: "default", dot: "bg-primary animate-pulse", icon: Activity },
  3: { label: "Completed", variant: "default", dot: "bg-emerald-500", icon: CheckCircle2 },
  4: { label: "Failed", variant: "destructive", dot: "bg-destructive", icon: XCircle },
  5: { label: "Cancelled", variant: "outline", dot: "bg-muted-foreground", icon: XCircle },
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

export function RunsPage() {
  const [suites, setSuites] = useState<Suite[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const pageSize = 20

  const load = useCallback(async () => {
    try {
      const res = await api.listSuites({ limit: BigInt(pageSize), offset: BigInt(page * pageSize) })
      setSuites(res.suites)
      setTotal(Number(res.total))
    } catch { /* ignore */ }
  }, [page])

  useEffect(() => { load() }, [load])

  const totalPages = Math.ceil(total / pageSize)

  // Stats from current page data
  const running = suites.filter(s => s.status === 2).length
  const completed = suites.filter(s => s.status === 3).length
  const failed = suites.filter(s => s.status === 4).length

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Test Runs</h1>
          <p className="text-muted-foreground text-sm mt-0.5">
            <span className="font-mono text-xs">{total}</span> suite(s) total
          </p>
        </div>
        <Button size="sm" asChild>
          <Link to="/tests/new"><Plus className="w-3.5 h-3.5 mr-1.5" />New Test</Link>
        </Button>
      </div>

      {/* Stats strip */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: "Total", value: total, icon: Activity, color: "text-primary" },
          { label: "Running", value: running, icon: Timer, color: "text-primary" },
          { label: "Completed", value: completed, icon: CheckCircle2, color: "text-emerald-500" },
          { label: "Failed", value: failed, icon: XCircle, color: "text-destructive" },
        ].map(stat => (
          <Card key={stat.label}>
            <CardContent className="pt-4 pb-3">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">{stat.label}</p>
                  <p className="text-2xl font-semibold mt-1 font-mono">{stat.value}</p>
                </div>
                <div className="w-8 h-8 rounded-lg bg-muted/60 flex items-center justify-center">
                  <stat.icon className={`w-4 h-4 ${stat.color}`} />
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Timeline feed */}
      {suites.length === 0 ? (
        <Card>
          <CardContent className="py-16 flex flex-col items-center">
            <div className="w-12 h-12 rounded-2xl bg-muted flex items-center justify-center mb-3">
              <PlayCircle className="w-6 h-6 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium mb-1">No test suites executed yet</p>
            <p className="text-xs text-muted-foreground mb-4">Run your first test to see results here</p>
            <Button size="sm" asChild>
              <Link to="/tests/new"><Plus className="w-3.5 h-3.5 mr-1.5" />New Test</Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="relative">
          {/* Timeline line */}
          <div className="absolute left-[19px] top-0 bottom-0 w-px bg-border/60" />

          <div className="space-y-3">
            {suites.map(s => {
              const sc = statusConfig[s.status] ?? statusConfig[0]
              const StatusIcon = sc.icon
              return (
                <Link key={s.suiteId} to={`/suites/${s.suiteId}`}
                  className="group relative flex gap-4 pl-0 hover:bg-transparent">
                  {/* Timeline dot */}
                  <div className="relative z-10 flex items-start pt-4">
                    <div className={`w-[38px] h-[38px] rounded-xl border flex items-center justify-center shrink-0 ${
                      s.status === 2 ? "bg-primary/10 border-primary/30" :
                      s.status === 3 ? "bg-emerald-500/10 border-emerald-500/30" :
                      s.status === 4 ? "bg-destructive/10 border-destructive/30" :
                      "bg-muted border-border"
                    }`}>
                      <StatusIcon className={`w-4 h-4 ${
                        s.status === 2 ? "text-primary" :
                        s.status === 3 ? "text-emerald-500" :
                        s.status === 4 ? "text-destructive" :
                        "text-muted-foreground"
                      }`} />
                    </div>
                  </div>

                  {/* Card */}
                  <Card className="flex-1 group-hover:border-primary/30 transition-colors">
                    <CardContent className="pt-4 pb-3">
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex items-center gap-2.5">
                          <code className="text-xs font-mono text-muted-foreground">{s.suiteId.slice(0, 12)}</code>
                          <StatusBadge status={s.status} />
                        </div>
                        <ArrowRight className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity mt-0.5" />
                      </div>

                      <div className="flex items-center gap-6 text-xs text-muted-foreground">
                        <span className="font-mono">
                          <span className="font-medium text-foreground">{s.testSuite?.tests.length ?? 0}</span> test(s)
                        </span>
                        {s.durationMs ? (
                          <span className="flex items-center gap-1 font-mono">
                            <Timer className="w-3 h-3" />
                            {(Number(s.durationMs) / 1000).toFixed(1)}s
                          </span>
                        ) : null}
                        <span className="flex items-center gap-1">
                          <Clock className="w-3 h-3" />
                          {s.createdAt ? new Date(Number(s.createdAt.seconds) * 1000).toLocaleString() : "-"}
                        </span>
                      </div>
                    </CardContent>
                  </Card>
                </Link>
              )
            })}
          </div>
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex justify-between items-center pt-2">
          <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(p => p - 1)}>
            Previous
          </Button>
          <span className="text-xs text-muted-foreground font-mono">
            {page + 1} / {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={(page + 1) * pageSize >= total} onClick={() => setPage(p => p + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
