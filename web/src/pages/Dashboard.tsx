import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getHealth } from "@/api/client";
import { WSConnection } from "@/api/ws";
import type { NodeStatus, WSMessage, Snapshot } from "@/api/types";
import { RunCard } from "@/components/RunCard";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Play, Database, Activity, Zap } from "lucide-react";

interface RunEntry {
  runID: string;
  nodes: NodeStatus[];
  lastUpdate: number;
}

export function Dashboard() {
  const [healthy, setHealthy] = useState<boolean | null>(null);
  const [runs, setRuns] = useState<Map<string, RunEntry>>(new Map());

  // Health check
  useEffect(() => {
    let cancelled = false;
    async function check() {
      try {
        const res = await getHealth();
        if (!cancelled) setHealthy(res.status === "ok");
      } catch {
        if (!cancelled) setHealthy(false);
      }
    }
    check();
    const interval = setInterval(check, 10000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  // WebSocket for live updates
  useEffect(() => {
    const ws = new WSConnection();

    ws.onMessage((msg: WSMessage) => {
      if (msg.type === "report") {
        const report = msg.payload as {
          command_id: string;
          run_id?: string;
          status: string;
          node_id?: string;
        };
        // We collect reports to build a view of active runs
        // This is a simplified approach -- real implementation would track per-run
        const runID = report.run_id || "unknown";
        setRuns((prev) => {
          const next = new Map(prev);
          const existing = next.get(runID) || {
            runID,
            nodes: [],
            lastUpdate: Date.now(),
          };
          existing.lastUpdate = Date.now();

          if (report.node_id) {
            const idx = existing.nodes.findIndex(
              (n) => n.id === report.node_id
            );
            const nodeStatus: NodeStatus = {
              id: report.node_id,
              status:
                report.status === "ok"
                  ? "done"
                  : report.status === "error"
                    ? "failed"
                    : "pending",
            };
            if (idx >= 0) {
              existing.nodes[idx] = nodeStatus;
            } else {
              existing.nodes.push(nodeStatus);
            }
          }

          next.set(runID, existing);
          return next;
        });
      }
    });

    ws.connect();
    return () => ws.disconnect();
  }, []);

  // Update health indicator in sidebar
  useEffect(() => {
    const dot = document.getElementById("health-indicator");
    const text = document.getElementById("health-text");
    if (dot) {
      dot.className = `w-2 h-2 ${
        healthy === null
          ? "bg-warning"
          : healthy
            ? "bg-success"
            : "bg-destructive"
      }`;
    }
    if (text) {
      text.textContent = healthy === null
        ? "Checking..."
        : healthy
          ? "Connected"
          : "Offline";
    }
  }, [healthy]);

  const runList = Array.from(runs.values()).sort(
    (a, b) => b.lastUpdate - a.lastUpdate
  );

  const quickStarts = [
    {
      label: "Postgres Single",
      kind: "postgres",
      preset: "single",
      icon: Database,
    },
    { label: "Postgres HA", kind: "postgres", preset: "ha", icon: Database },
    { label: "MySQL Single", kind: "mysql", preset: "single", icon: Database },
    { label: "MySQL Group", kind: "mysql", preset: "group", icon: Database },
    {
      label: "Picodata Cluster",
      kind: "picodata",
      preset: "cluster",
      icon: Zap,
    },
  ];

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Monitor active runs and launch new tests
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={healthy ? "success" : healthy === false ? "destructive" : "warning"}>
            <Activity className="h-3 w-3 mr-1" />
            {healthy === null ? "checking" : healthy ? "healthy" : "offline"}
          </Badge>
          <Link to="/runs/new">
            <Button size="sm">
              <Play className="h-3.5 w-3.5" />
              New Run
            </Button>
          </Link>
        </div>
      </div>

      {/* Quick start */}
      <div>
        <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
          Quick Start
        </h2>
        <div className="grid grid-cols-5 gap-3">
          {quickStarts.map((qs) => (
            <Link
              key={qs.label}
              to={`/runs/new?kind=${qs.kind}&preset=${qs.preset}`}
            >
              <Card className="hover:border-primary/50 transition-colors cursor-pointer">
                <CardContent className="p-3 flex items-center gap-2">
                  <qs.icon className="h-4 w-4 text-primary" />
                  <span className="text-xs font-medium">{qs.label}</span>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </div>

      {/* Active runs */}
      <div>
        <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
          Active Runs
        </h2>
        {runList.length === 0 ? (
          <Card>
            <CardContent className="p-8 text-center">
              <p className="text-sm text-muted-foreground">
                No active runs. Start one from the quick start panel above or{" "}
                <Link to="/runs/new" className="text-primary hover:underline">
                  create a new run
                </Link>
                .
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-3 gap-3">
            {runList.map((run) => (
              <RunCard
                key={run.runID}
                runID={run.runID}
                nodes={run.nodes}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
