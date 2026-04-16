import { useEffect, useState, useMemo } from "react";
import { useParams } from "react-router-dom";
import { getSharedRun } from "@/api/client";
import type { Snapshot, NodeStatus, RunConfig } from "@/api/types";
import { MetricsPanel } from "@/components/MetricsPanel";
import { RunOverview } from "@/components/RunOverview";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { AlertCircle, Loader2, Share2 } from "lucide-react";

export function SharedRun() {
  const { token } = useParams<{ token: string }>();
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [metrics, setMetrics] = useState<MetricSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [runId, setRunId] = useState("");
  const [createdAt, setCreatedAt] = useState("");

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    getSharedRun(token)
      .then((data) => {
        setRunId(data.run_id);
        setCreatedAt(data.created_at);
        setSnapshot(data.snapshot as Snapshot);
        const m = data.metrics as { metrics?: MetricSummary[] };
        setMetrics(m?.metrics || []);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load shared run"))
      .finally(() => setLoading(false));
  }, [token]);

  const nodes: NodeStatus[] = snapshot?.nodes || [];
  const config = useMemo<RunConfig | null>(() => {
    const rc = snapshot?.state?.run_config;
    if (!rc) return null;
    try {
      return typeof rc === "string" ? JSON.parse(rc) : (rc as unknown as RunConfig);
    } catch {
      return null;
    }
  }, [snapshot]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen gap-2">
        <Loader2 className="h-5 w-5 text-zinc-500 animate-spin" />
        <span className="text-sm text-zinc-500 font-mono">Loading shared run...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-screen gap-2">
        <AlertCircle className="h-5 w-5 text-red-400" />
        <span className="text-sm text-red-400 font-mono">{error}</span>
      </div>
    );
  }

  const dbKind = config?.database?.kind || "";
  const dbVersion = config?.database?.version || "";

  return (
    <div className="h-screen flex flex-col bg-[#0a0a0a] text-zinc-100">
      {/* Header */}
      <div className="flex items-center justify-between px-5 py-3 border-b border-zinc-800/50">
        <div className="flex items-center gap-3">
          <Share2 className="h-4 w-4 text-primary" />
          <h1 className="text-base font-semibold font-mono">Shared Run</h1>
          <span className="text-xs font-mono text-zinc-500">{runId}</span>
          {dbKind && (
            <span className="text-xs font-mono text-zinc-600">{dbKind} {dbVersion}</span>
          )}
        </div>
        <span className="text-[10px] font-mono text-zinc-600">
          Shared {createdAt ? new Date(createdAt).toLocaleDateString() : ""}
        </span>
      </div>

      {/* Content */}
      <div className="flex-1 min-h-0 p-5">
        <Tabs defaultValue="metrics" className="h-full flex flex-col">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="metrics">Metrics</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="flex-1">
            <Card className="h-[calc(100vh-11rem)]">
              <CardContent className="p-0 h-full">
                <RunOverview
                  nodes={nodes}
                  snapshot={snapshot}
                  runStatus={nodes.some(n => n.status === "failed") ? "failed" : "completed"}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="metrics" className="flex-1">
            <Card className="h-[calc(100vh-11rem)]">
              <CardContent className="p-4 h-full overflow-auto">
                {metrics && metrics.length > 0 ? (
                  <FrozenMetrics metrics={metrics} />
                ) : (
                  <div className="text-xs text-zinc-600 font-mono">No metrics data.</div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}

interface MetricSummary {
  key: string;
  name: string;
  unit: string;
  avg: number;
  min: number;
  max: number;
  last: number;
}

function fmtVal(v: number, unit: string): string {
  if (unit === "%" || unit === "pct") return v.toFixed(1) + "%";
  if (v >= 1_000_000) return (v / 1_000_000).toFixed(1) + "M";
  if (v >= 1_000) return (v / 1_000).toFixed(1) + "K";
  return v < 10 ? v.toFixed(2) : v.toFixed(0);
}

const METRIC_CATEGORIES: Record<string, string[]> = {
  "Database": ["db_qps", "db_tps", "db_latency_p99", "db_connections", "db_repl_lag", "db_errors", "db_sessions", "db_tablets", "db_raft_commit", "db_tables"],
  "System": ["cpu_usage", "memory_usage", "disk_read", "disk_write", "net_rx", "net_tx"],
  "Stroppy": ["stroppy_vus", "stroppy_ops", "stroppy_iter_p99", "stroppy_query_rate", "stroppy_latency_p99", "stroppy_errors"],
};

function FrozenMetrics({ metrics }: { metrics: MetricSummary[] }) {
  const byKey = new Map(metrics.map((m) => [m.key, m]));

  return (
    <div className="space-y-6">
      {Object.entries(METRIC_CATEGORIES).map(([cat, keys]) => {
        const vals = keys.map((k) => byKey.get(k)).filter(Boolean) as MetricSummary[];
        if (vals.length === 0) return null;
        return (
          <div key={cat}>
            <h3 className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2">{cat}</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {vals.map((m) => (
                <div key={m.key} className="border border-zinc-800/60 p-3 bg-zinc-900/30">
                  <div className="text-[10px] font-mono text-zinc-500 truncate" title={m.key}>{m.name}</div>
                  <div className="text-lg font-mono font-semibold text-zinc-200 mt-1">
                    {fmtVal(m.avg, m.unit)}
                  </div>
                  <div className="flex gap-3 mt-1 text-[9px] font-mono text-zinc-600">
                    <span>min {fmtVal(m.min, m.unit)}</span>
                    <span>max {fmtVal(m.max, m.unit)}</span>
                    <span>last {fmtVal(m.last, m.unit)}</span>
                  </div>
                  {m.unit && <div className="text-[9px] font-mono text-zinc-700 mt-0.5">{m.unit}</div>}
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
