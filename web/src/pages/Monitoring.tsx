import { useEffect, useState } from "react";
import { getGrafanaSettings } from "@/api/client";
import type { GrafanaSettings } from "@/api/types";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertCircle } from "lucide-react";

export function Monitoring() {
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [runID, setRunID] = useState("");
  const [selectedDashboard, setSelectedDashboard] = useState("overview");

  useEffect(() => {
    getGrafanaSettings()
      .then(setGrafana)
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Failed to load Grafana settings")
      );
  }, []);

  if (error) {
    return (
      <div className="p-6">
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      </div>
    );
  }

  if (!grafana || !grafana.embed_enabled) {
    return (
      <div className="p-6">
        <h1 className="text-lg font-semibold">Monitoring</h1>
        <p className="text-sm text-muted-foreground mt-2">
          Grafana embedding is not enabled. Configure it in Settings.
        </p>
      </div>
    );
  }

  const dashboards = grafana.dashboards || {};
  const uid = dashboards[selectedDashboard] || dashboards["overview"] || "";

  const dashboardURL = runID
    ? `${grafana.url}/d/${uid}?var-run_id=${runID}&kiosk&theme=dark`
    : `${grafana.url}/d/${uid}?kiosk&theme=dark`;

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-semibold">Monitoring</h1>
        <p className="text-sm text-muted-foreground">
          Full Grafana dashboard view
        </p>
      </div>

      <div className="flex items-end gap-4">
        <div className="space-y-2">
          <Label>Run ID (optional)</Label>
          <Input
            value={runID}
            onChange={(e) => setRunID(e.target.value)}
            placeholder="run-abc123"
            className="font-mono w-64"
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Grafana Dashboard</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="flex gap-2 p-3 border-b border-border">
            {Object.keys(dashboards).filter(n => n !== "compare").map(name => (
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
            src={dashboardURL}
            className="w-full h-[calc(100vh-280px)] border-0"
            title="Monitoring Dashboard"
          />
        </CardContent>
      </Card>
    </div>
  );
}
