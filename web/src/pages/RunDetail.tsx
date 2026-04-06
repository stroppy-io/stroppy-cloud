import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getRunStatus, getGrafanaSettings, deleteRun } from "@/api/client";
import type { Snapshot, NodeStatus, GrafanaSettings } from "@/api/types";
import { RunOverview } from "@/components/RunOverview";
import { LogStream } from "@/components/LogStream";
import { MetricsPanel } from "@/components/MetricsPanel";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { RefreshCw, AlertCircle, BarChart3, Trash2 } from "lucide-react";

// DB-specific dashboards — only show the one matching the run's database kind.
const DB_DASHBOARDS = ["postgres", "mysql", "picodata"];

function DashboardSelector({ grafana, runID, dbKind, startedAt, finishedAt }: {
  grafana: GrafanaSettings; runID: string; dbKind?: string; startedAt?: string; finishedAt?: string;
}) {
  const dashboards = grafana.dashboards || {};

  const visibleDashboards = Object.keys(dashboards).filter(name => {
    if (name === "compare" || name === "overview") return false;
    if (DB_DASHBOARDS.includes(name)) return name === dbKind;
    return true;
  });

  const [selectedDashboard, setSelectedDashboard] = useState(visibleDashboards[0] || "stroppy");

  const uid = dashboards[selectedDashboard] || "";

  // Build iframe src — memoize to avoid reloading iframe on every poll.
  // While run is in progress, use "now" as end time (Grafana auto-refreshes internally).
  // Once finished, lock the time range.
  const iframeSrc = useMemo(() => {
    let timeParams = "";
    if (startedAt && startedAt !== "0001-01-01T00:00:00Z") {
      const from = new Date(startedAt).getTime() - 30000;
      if (finishedAt && finishedAt !== "0001-01-01T00:00:00Z") {
        const to = new Date(finishedAt).getTime() + 60000;
        timeParams = `&from=${from}&to=${to}`;
      } else {
        // Run in progress — use relative "now" so Grafana auto-refreshes.
        timeParams = `&from=${from}&to=now&refresh=5s`;
      }
    }
    // var-run_id for system/db dashboards (vmagent external_labels), var-prefix for stroppy dashboard (K6 metric prefix = runID_)
    const prefix = runID.replace(/-/g, "_") + "_";
    return `${grafana.url}/d/${uid}?var-run_id=${runID}&var-prefix=${prefix}${timeParams}&kiosk&theme=dark`;
    // Only recalculate when dashboard, startedAt, or finishedAt changes — NOT on every poll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [grafana.url, uid, runID, startedAt, finishedAt]);

  return (
    <>
      <div className="flex gap-2 p-3 border-b border-border">
        {visibleDashboards.map(name => (
          <button
            type="button"
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
        src={iframeSrc}
        className="w-full border-0 h-[calc(100vh-14rem)]"
        title="Run Metrics Dashboard"
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
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

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="metrics">
            <BarChart3 className="h-3 w-3 mr-1" />
            Metrics
          </TabsTrigger>
          {grafana?.embed_enabled && (
            <TabsTrigger value="grafana">Grafana</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full">
              <RunOverview nodes={nodes} snapshot={snapshot} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full relative">
              <LogStream runID={id} />
            </CardContent>
          </Card>
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
              <CardContent className="p-0">
                <DashboardSelector
                  grafana={grafana}
                  runID={id || ""}
                  dbKind={(() => { try { const rc = snapshot?.state?.run_config; if (!rc) return undefined; const cfg = typeof rc === "string" ? JSON.parse(rc) : rc; return cfg?.database?.kind; } catch { return undefined; } })()}
                  startedAt={snapshot?.started_at}
                  finishedAt={snapshot?.finished_at}
                />
              </CardContent>
            </Card>
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
