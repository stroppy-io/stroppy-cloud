import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getRunStatus, getGrafanaSettings, deleteRun, cancelRun, createShareLink } from "@/api/client";
import { ALL_DB_KINDS, type Snapshot, type NodeStatus, type GrafanaSettings, type RunConfig } from "@/api/types";
import { RunOverview } from "@/components/RunOverview";
import { LogStream } from "@/components/LogStream";
import { MetricsPanel } from "@/components/MetricsPanel";
import { TopologyFlow } from "@/components/TopologyFlow";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { RefreshCw, AlertCircle, Trash2, StopCircle, Share2, Check } from "lucide-react";

// DB-specific dashboards — only show the one matching the run's database kind.
const DB_DASHBOARDS = ALL_DB_KINDS;

function DashboardSelector({ grafana, runID, dbKind, startedAt, finishedAt }: {
  grafana: GrafanaSettings; runID: string; dbKind?: string; startedAt?: string; finishedAt?: string;
}) {
  const dashboards = grafana.dashboards || {};

  const visibleDashboards = Object.keys(dashboards).filter(name => {
    if (name === "compare" || name === "overview") return false;
    if ((DB_DASHBOARDS as string[]).includes(name)) return name === dbKind;
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
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [sharing, setSharing] = useState(false);
  const [, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);
  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;

  const fetchStatus = useCallback(async () => {
    if (!id) return;
    try {
      const snap = await getRunStatus(id);
      setSnapshot(snap);
      setError(null);
    } catch (err) {
      // Run may not be ready yet (DAG still building) — don't show error immediately.
      if (!snapshotRef.current) {
        setError(null); // suppress error, keep showing loading
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch status");
      }
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

  const [cancelling, setCancelling] = useState(false);

  async function handleCancel() {
    if (!id || cancelling) return;
    setCancelling(true);
    try {
      await cancelRun(id);
      // Don't reset cancelling — polling will show the run finishing via teardown.
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel run");
      setCancelling(false);
    }
  }

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

  const nodes: NodeStatus[] = snapshot?.nodes || [];
  const hasFailed = nodes.some((n) => n.status === "failed");
  const hasCancelled = nodes.some((n) => n.status === "cancelled");
  const hasRunning = nodes.some((n) => n.status === "running");
  const hasPending = nodes.some((n) => n.status === "pending");
  const inTeardown = nodes.some((n) => n.id === "teardown" && (n.status === "running" || n.status === "done"));

  // Run is finished when no nodes are pending or running.
  const isFinished = snapshot
    ? snapshot.nodes.length > 0 && !hasPending && !hasRunning
    : false;

  // Cancelled = has cancelled nodes (proper FSM state from backend).
  const isCancelled = hasCancelled;

  const runConfig = useMemo<RunConfig | null>(() => {
    const rc = snapshot?.state?.run_config;
    if (!rc) return null;
    try {
      return typeof rc === "string" ? JSON.parse(rc) : (rc as unknown as RunConfig);
    } catch {
      return null;
    }
  }, [snapshot]);

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-lg font-semibold font-mono">{id}</h1>
            <p className="text-sm text-muted-foreground">Run detail view</p>
          </div>
          {/* Run status badge */}
          {snapshot && (
            isCancelled ? (
              <Badge className="bg-zinc-500/20 text-zinc-400 border-zinc-500/30">Cancelled</Badge>
            ) : cancelling ? (
              <Badge className="bg-amber-500/20 text-amber-400 border-amber-500/30 animate-pulse">Cancelling...</Badge>
            ) : !isFinished ? (
              <Badge className="bg-blue-500/20 text-blue-400 border-blue-500/30 animate-pulse">Running</Badge>
            ) : hasFailed ? (
              <Badge variant="destructive">Failed</Badge>
            ) : (
              <Badge variant="success">Completed</Badge>
            )
          )}
        </div>
        <div className="flex items-center gap-3">
          <Button variant="outline" size="sm" onClick={fetchStatus}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>

          {!isFinished && snapshot && !inTeardown && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleCancel}
              disabled={cancelling}
              className="border-amber-800 text-amber-400 hover:bg-amber-500/10"
            >
              <StopCircle className="h-3.5 w-3.5" />
              {cancelling ? "Cancelling..." : "Cancel Run"}
            </Button>
          )}

          {isFinished && (
            <>
              <Button
                variant="outline"
                size="sm"
                disabled={sharing}
                className="border-primary/40 text-primary hover:bg-primary/10"
                onClick={async () => {
                  if (shareUrl) {
                    navigator.clipboard.writeText(window.location.origin + shareUrl);
                    return;
                  }
                  setSharing(true);
                  try {
                    const res = await createShareLink(id!);
                    setShareUrl(res.url);
                    navigator.clipboard.writeText(window.location.origin + res.url);
                  } catch (err) {
                    setError(err instanceof Error ? err.message : "Failed to create share link");
                  } finally {
                    setSharing(false);
                  }
                }}
              >
                {shareUrl ? <Check className="h-3.5 w-3.5" /> : <Share2 className="h-3.5 w-3.5" />}
                {sharing ? "Sharing..." : shareUrl ? "Copied!" : "Share"}
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleDelete}
                disabled={deleting}
              >
                <Trash2 className="h-3.5 w-3.5" />
                {deleting ? "Deleting..." : "Delete"}
              </Button>
            </>
          )}
        </div>
      </div>

      {!snapshot && !error && (
        <div className="text-sm text-muted-foreground">Loading run status...</div>
      )}

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <Tabs defaultValue={(() => {
        const params = new URLSearchParams(window.location.search);
        if (params.get("tab")) return params.get("tab")!;
        if (window.location.hash.startsWith("#L")) return "logs";
        return "overview";
      })()}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="topology">Topology</TabsTrigger>
          <div className="w-px h-4 bg-zinc-800 mx-1" />
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          {grafana?.embed_enabled && (
            <TabsTrigger value="grafana">Grafana</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full">
              <RunOverview nodes={nodes} snapshot={snapshot} runStatus={isCancelled ? "cancelled" : cancelling ? "cancelling" : !isFinished ? "running" : hasFailed ? "failed" : "completed"} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="topology">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full">
              <TopologyFlow config={runConfig} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs" forceMount className="data-[state=inactive]:hidden">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full relative">
              <LogStream runID={id} snapshot={snapshot} />
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
