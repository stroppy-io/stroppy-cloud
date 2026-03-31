import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getRunStatus, getGrafanaSettings, deleteRun } from "@/api/client";
import type { Snapshot, NodeStatus, GrafanaSettings } from "@/api/types";
import { DagGraph } from "@/components/DagGraph";
import { LogStream } from "@/components/LogStream";
import { MetricsPanel } from "@/components/MetricsPanel";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { RefreshCw, AlertCircle, BarChart3, Trash2 } from "lucide-react";

// DB-specific dashboards — only show the one matching the run's database kind.
const DB_DASHBOARDS = ["postgres", "mysql", "picodata"];

function DashboardSelector({ grafana, runID, dbKind }: { grafana: GrafanaSettings; runID: string; dbKind?: string }) {
  const [selectedDashboard, setSelectedDashboard] = useState("overview");

  const dashboards = grafana.dashboards || {};

  // Filter: show overview, system, stroppy always. Show only the matching DB dashboard.
  const visibleDashboards = Object.keys(dashboards).filter(name => {
    if (name === "compare") return false;
    if (DB_DASHBOARDS.includes(name)) return name === dbKind;
    return true;
  });

  const uid = dashboards[selectedDashboard] || dashboards["overview"] || "";

  return (
    <>
      <div className="flex gap-2 p-3 border-b border-border">
        {visibleDashboards.map(name => (
          <button
            key={name}
            onClick={() => setSelectedDashboard(name)}
            className={`px-3 py-1 text-xs font-mono border ${
              selectedDashboard === name
                ? "border-primary text-primary bg-primary/5"
                : "border-zinc-800 text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {name}
          </button>
        ))}
      </div>
      <iframe
        src={`${grafana.url}/d/${uid}?var-run_id=${runID}&kiosk&theme=dark`}
        className="w-full h-[600px] border-0"
        title="Run Metrics Dashboard"
      />
    </>
  );
}

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);
  const fetchStatus = useCallback(async () => {
    if (!id) return;
    try {
      const snap = await getRunStatus(id);
      setSnapshot(snap);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch status");
    } finally {
      setLoading(false);
    }
  }, [id]);

  const finishedRef = useRef(false);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(() => {
      if (finishedRef.current) {
        clearInterval(interval);
        return;
      }
      fetchStatus();
    }, 3000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Track whether run is finished to stop polling.
  useEffect(() => {
    if (snapshot && snapshot.nodes.length > 0) {
      const hasPending = snapshot.nodes.some(n => n.status === "pending");
      finishedRef.current = !hasPending;
    }
  }, [snapshot]);

  useEffect(() => {
    getGrafanaSettings()
      .then(setGrafana)
      .catch(() => setGrafana(null));
  }, []);

  async function handleDelete() {
    if (!id) return;
    if (!confirm(`Delete run "${id}"? This will also remove its Docker resources.`)) return;
    setDeleting(true);
    try {
      await deleteRun(id);
      navigate("/runs");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete run");
      setDeleting(false);
    }
  }

  const isFinished = snapshot
    ? snapshot.nodes.length > 0 && !snapshot.nodes.some((n) => n.status === "pending")
    : false;

  const nodes: NodeStatus[] = snapshot?.nodes || [];
  const failedNodes = nodes.filter((n) => n.status === "failed");
  const doneCount = nodes.filter((n) => n.status === "done").length;
  const pendingCount = nodes.filter((n) => n.status === "pending").length;
  const hasFailed = failedNodes.length > 0;

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold font-mono">{id}</h1>
          <p className="text-sm text-muted-foreground">Run detail view</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex gap-2 text-xs">
            <Badge variant="success">{doneCount} done</Badge>
            <Badge variant="pending">{pendingCount} pending</Badge>
            {hasFailed && (
              <Badge variant="destructive">{failedNodes.length} failed</Badge>
            )}
          </div>

          <Button variant="outline" size="sm" onClick={fetchStatus}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>

          {isFinished && (
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDelete}
              disabled={deleting}
            >
              <Trash2 className="h-3.5 w-3.5" />
              {deleting ? "Deleting..." : "Delete"}
            </Button>
          )}
        </div>
      </div>

      {loading && !snapshot && (
        <div className="text-sm text-muted-foreground">Loading run status...</div>
      )}

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <Tabs defaultValue="dag">
        <TabsList>
          <TabsTrigger value="dag">DAG</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="phases">Phases</TabsTrigger>
          <TabsTrigger value="metrics">
            <BarChart3 className="h-3 w-3 mr-1" />
            Metrics
          </TabsTrigger>
          {grafana?.embed_enabled && (
            <TabsTrigger value="grafana">Grafana</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="dag">
          <Card>
            <CardHeader>
              <CardTitle>Execution DAG</CardTitle>
            </CardHeader>
            <CardContent>
              <DagGraph nodes={nodes} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs">
          <Card className="h-[calc(100vh-280px)]">
            <CardContent className="p-0 h-full relative">
              <LogStream runID={id} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="phases">
          <Card>
            <CardHeader>
              <CardTitle>Phase Timeline</CardTitle>
            </CardHeader>
            <CardContent>
              {nodes.length === 0 ? (
                <p className="text-sm text-muted-foreground">No phase data yet.</p>
              ) : (
                <div className="space-y-1">
                  {nodes.map((node) => (
                    <div
                      key={node.id}
                      className="flex items-center gap-3 py-1.5 border-b border-border/30 last:border-0"
                    >
                      <div
                        className={`w-2 h-2 shrink-0 ${
                          node.status === "done"
                            ? "bg-success"
                            : node.status === "failed"
                              ? "bg-destructive"
                              : "bg-pending"
                        }`}
                      />
                      <span className="font-mono text-xs w-48 shrink-0">{node.id}</span>
                      <div className="flex-1">
                        <div
                          className={`h-4 ${
                            node.status === "done"
                              ? "bg-success/20"
                              : node.status === "failed"
                                ? "bg-destructive/20"
                                : "bg-muted"
                          }`}
                          style={{
                            width: node.status === "done" ? "100%" : node.status === "failed" ? "60%" : "0%",
                          }}
                        />
                      </div>
                      <Badge
                        variant={
                          node.status === "done" ? "success" : node.status === "failed" ? "destructive" : "pending"
                        }
                        className="shrink-0"
                      >
                        {node.status}
                      </Badge>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {hasFailed && (
            <Card className="mt-4 border-destructive/30">
              <CardHeader>
                <CardTitle className="text-destructive">Errors</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {failedNodes.map((node) => (
                  <div key={node.id} className="p-3 bg-destructive/5 border border-destructive/20">
                    <div className="text-xs font-mono font-medium text-destructive mb-1">{node.id}</div>
                    <div className="text-xs font-mono text-destructive/80">{node.error || "Unknown error"}</div>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* Metrics tab — direct API data */}
        <TabsContent value="metrics">
          {id ? (
            <MetricsPanel runID={id} startedAt={snapshot?.started_at} finishedAt={snapshot?.finished_at} />
          ) : (
            <p className="text-sm text-muted-foreground p-4">No run ID</p>
          )}
        </TabsContent>

        {/* Grafana embed tab */}
        {grafana?.embed_enabled && (
          <TabsContent value="grafana">
            <Card>
              <CardHeader>
                <CardTitle>Grafana Dashboard</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <DashboardSelector
                  grafana={grafana}
                  runID={id || ""}
                  dbKind={(() => { try { const rc = snapshot?.state?.run_config; if (!rc) return undefined; const cfg = typeof rc === "string" ? JSON.parse(rc) : rc; return cfg?.database?.kind; } catch { return undefined; } })()}
                />
              </CardContent>
            </Card>
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
