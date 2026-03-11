import { useEffect, useState, useCallback } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  ChevronLeft, Loader2, StopCircle, RefreshCw,
  Activity, Server, Clock, Target, Timer, CheckCircle2, XCircle
} from "lucide-react"
import type { Suite, Run } from "@/proto/api/api_pb.ts"

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

export function SuiteDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [suite, setSuite] = useState<Suite | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!id) return
    try {
      const res = await api.getSuite({ suiteId: id })
      setSuite(res.suite ?? null)
    } catch {
      navigate("/runs")
    }
    setLoading(false)
  }, [id, navigate])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    if (!suite || (suite.status !== 1 && suite.status !== 2)) return
    const interval = setInterval(load, 3000)
    return () => clearInterval(interval)
  }, [suite, load])

  const handleCancel = async (runId: string) => {
    try {
      await api.cancelRun({ runId })
      load()
    } catch (err) {
      alert(err instanceof Error ? err.message : "Cancel failed")
    }
  }

  if (loading) {
    return <div className="flex items-center justify-center h-64"><Loader2 className="w-5 h-5 animate-spin text-muted-foreground" /></div>
  }

  if (!suite) return null

  const isActive = suite.status === 1 || suite.status === 2

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate("/runs")}>
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-semibold tracking-tight">Suite</h1>
              <code className="text-xs text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">{suite.suiteId.slice(0, 12)}</code>
              <StatusBadge status={suite.status} />
            </div>
            <p className="text-muted-foreground text-sm mt-0.5">
              {suite.testSuite?.tests.length ?? 0} test(s)
              {suite.durationMs ? ` \u00b7 ${(Number(suite.durationMs) / 1000).toFixed(1)}s` : ""}
            </p>
          </div>
        </div>
        {isActive && (
          <Button variant="outline" size="sm" onClick={load}>
            <RefreshCw className="w-3.5 h-3.5 mr-1.5" />Refresh
          </Button>
        )}
      </div>

      {/* Two-column layout: sidebar summary + main content */}
      <div className="grid grid-cols-[280px_1fr] gap-6">
        {/* Left sidebar: summary */}
        <div className="space-y-4">
          <Card>
            <CardContent className="pt-5 pb-4 space-y-4">
              <div>
                <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                  <Activity className="w-3 h-3" />Status
                </span>
                <div className="mt-2"><StatusBadge status={suite.status} /></div>
              </div>
              <Separator />
              <div>
                <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                  <Server className="w-3 h-3" />Tests
                </span>
                <p className="text-2xl font-semibold font-mono mt-1">{suite.testSuite?.tests.length ?? 0}</p>
              </div>
              <Separator />
              <div>
                <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                  <Target className="w-3 h-3" />Target
                </span>
                <p className="text-sm font-mono mt-1">{suite.target === 1 ? "Docker" : suite.target === 2 ? "Yandex Cloud" : "Unspecified"}</p>
              </div>
              <Separator />
              <div>
                <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                  <Clock className="w-3 h-3" />Created
                </span>
                <p className="text-sm font-mono mt-1">{suite.createdAt ? new Date(Number(suite.createdAt.seconds) * 1000).toLocaleString() : "-"}</p>
              </div>
              {suite.durationMs ? (
                <>
                  <Separator />
                  <div>
                    <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                      <Timer className="w-3 h-3" />Duration
                    </span>
                    <p className="text-sm font-mono mt-1">{(Number(suite.durationMs) / 1000).toFixed(1)}s</p>
                  </div>
                </>
              ) : null}
            </CardContent>
          </Card>

          {/* Test definitions in sidebar */}
          <Card>
            <CardContent className="pt-5 pb-4">
              <h3 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-3">Test Definitions</h3>
              <div className="space-y-2">
                {suite.testSuite?.tests.map((test, i) => (
                  <div key={i} className="p-2.5 rounded-lg border border-border/40 bg-muted/20">
                    <div className="flex items-center gap-2 mb-1">
                      <div className="w-5 h-5 rounded-md bg-primary/10 flex items-center justify-center text-[9px] font-mono font-semibold text-primary">{i + 1}</div>
                      <span className="font-medium text-[12px] truncate">{test.name || `test-${i + 1}`}</span>
                    </div>
                    {test.description && <p className="text-[10px] text-muted-foreground ml-7 mb-1">{test.description}</p>}
                    <div className="flex gap-2 ml-7 text-[10px] text-muted-foreground font-mono">
                      <span>{test.stroppyHardware?.cores ?? 0}C</span>
                      <span>/</span>
                      <span>{test.stroppyHardware?.memory ?? 0}GB</span>
                      <span>/</span>
                      <span>{test.stroppyHardware?.disk ?? 0}GB</span>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Right main content: runs + error */}
        <div className="space-y-4">
          {suite.errorMessage && (
            <Card className="border-destructive/50">
              <CardContent className="pt-5 pb-4">
                <div className="flex items-center gap-1.5 mb-2">
                  <span className="w-1.5 h-1.5 rounded-full bg-destructive" />
                  <span className="text-[11px] font-medium text-destructive uppercase tracking-wider">Error</span>
                </div>
                <pre className="text-sm text-destructive/80 whitespace-pre-wrap font-mono">{suite.errorMessage}</pre>
              </CardContent>
            </Card>
          )}

          <div>
            <h3 className="text-sm font-medium mb-3">Runs</h3>
            {(!suite.runs || suite.runs.length === 0) ? (
              <Card>
                <CardContent className="py-12 flex flex-col items-center">
                  <div className="w-10 h-10 rounded-xl bg-muted flex items-center justify-center mb-3">
                    <Activity className="w-5 h-5 text-muted-foreground" />
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {isActive ? "Waiting for runs to start..." : "No runs recorded."}
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="space-y-3">
                {suite.runs.map((run: Run) => {
                  const rc = statusConfig[run.status] ?? statusConfig[0]
                  const RunIcon = rc.icon
                  return (
                    <Card key={run.runId} className="group hover:border-primary/30 transition-colors">
                      <CardContent className="pt-4 pb-3">
                        <div className="flex items-start justify-between">
                          <div className="flex items-start gap-3">
                            <div className={`w-9 h-9 rounded-lg flex items-center justify-center shrink-0 ${
                              run.status === 2 ? "bg-primary/10" :
                              run.status === 3 ? "bg-emerald-500/10" :
                              run.status === 4 ? "bg-destructive/10" :
                              "bg-muted"
                            }`}>
                              <RunIcon className={`w-4 h-4 ${
                                run.status === 2 ? "text-primary" :
                                run.status === 3 ? "text-emerald-500" :
                                run.status === 4 ? "text-destructive" :
                                "text-muted-foreground"
                              }`} />
                            </div>
                            <div>
                              <div className="flex items-center gap-2 mb-1">
                                <code className="text-xs font-mono">{run.runId.slice(0, 12)}</code>
                                <StatusBadge status={run.status} />
                              </div>
                              <div className="flex items-center gap-4 text-[11px] text-muted-foreground">
                                {run.durationMs ? (
                                  <span className="flex items-center gap-1 font-mono">
                                    <Timer className="w-3 h-3" />
                                    {(Number(run.durationMs) / 1000).toFixed(1)}s
                                  </span>
                                ) : null}
                                <span className="flex items-center gap-1 font-mono">
                                  <Clock className="w-3 h-3" />
                                  {run.createdAt ? new Date(Number(run.createdAt.seconds) * 1000).toLocaleString() : "-"}
                                </span>
                              </div>
                            </div>
                          </div>
                          {run.status === 2 && (
                            <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => handleCancel(run.runId)} title="Cancel">
                              <StopCircle className="w-3.5 h-3.5" />
                            </Button>
                          )}
                        </div>
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
