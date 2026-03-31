import { useState, useEffect } from "react";
import { compareRuns, getGrafanaSettings } from "@/api/client";
import type { ComparisonRow, GrafanaSettings } from "@/api/types";
import { MetricsDiff } from "@/components/MetricsDiff";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { AlertCircle, GitCompare, Table, BarChart3 } from "lucide-react";

export function Compare() {
  const [runA, setRunA] = useState("");
  const [runB, setRunB] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [rows, setRows] = useState<ComparisonRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<"table" | "dashboard">("table");
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);

  useEffect(() => {
    getGrafanaSettings()
      .then(setGrafana)
      .catch(() => setGrafana(null));
  }, []);

  async function handleCompare() {
    if (!runA || !runB || !start || !end) {
      setError("All fields are required");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const result = await compareRuns(runA, runB, start, end);
      setRows(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Comparison failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-semibold">Compare Runs</h1>
        <p className="text-sm text-muted-foreground">
          Compare metrics between two test runs
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Parameters</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Run A</Label>
              <Input
                value={runA}
                onChange={(e) => setRunA(e.target.value)}
                placeholder="run-abc123"
                className="font-mono"
              />
            </div>
            <div className="space-y-2">
              <Label>Run B</Label>
              <Input
                value={runB}
                onChange={(e) => setRunB(e.target.value)}
                placeholder="run-def456"
                className="font-mono"
              />
            </div>
            <div className="space-y-2">
              <Label>Start (RFC3339)</Label>
              <Input
                type="datetime-local"
                value={start}
                onChange={(e) => {
                  // Convert to RFC3339
                  const d = new Date(e.target.value);
                  setStart(d.toISOString());
                }}
              />
              {start && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  {start}
                </span>
              )}
            </div>
            <div className="space-y-2">
              <Label>End (RFC3339)</Label>
              <Input
                type="datetime-local"
                value={end}
                onChange={(e) => {
                  const d = new Date(e.target.value);
                  setEnd(d.toISOString());
                }}
              />
              {end && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  {end}
                </span>
              )}
            </div>
          </div>

          <div className="mt-4">
            <Button onClick={handleCompare} disabled={loading}>
              <GitCompare className="h-3.5 w-3.5" />
              {loading ? "Comparing..." : "Compare"}
            </Button>
          </div>

          {error && (
            <div className="mt-4 flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
              <AlertCircle className="h-4 w-4" />
              {error}
            </div>
          )}
        </CardContent>
      </Card>

      {(rows.length > 0 || (grafana?.embed_enabled && runA && runB)) && (
        <>
          {grafana?.embed_enabled && (
            <div className="flex gap-2">
              <Button
                variant={mode === "table" ? "default" : "outline"}
                size="sm"
                onClick={() => setMode("table")}
              >
                <Table className="h-3.5 w-3.5" />
                Table
              </Button>
              <Button
                variant={mode === "dashboard" ? "default" : "outline"}
                size="sm"
                onClick={() => setMode("dashboard")}
              >
                <BarChart3 className="h-3.5 w-3.5" />
                Dashboard
              </Button>
            </div>
          )}

          {mode === "table" && rows.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>
                  Results: {runA} vs {runB}
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <MetricsDiff rows={rows} />
              </CardContent>
            </Card>
          )}

          {mode === "dashboard" && grafana?.embed_enabled && runA && runB && (
            <Card>
              <CardHeader>
                <CardTitle>
                  Dashboard: {runA} vs {runB}
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <iframe
                  src={`${grafana.url}/d/${grafana.dashboards?.compare}?var-run_a=${runA}&var-run_b=${runB}&kiosk&theme=dark`}
                  className="w-full h-[700px] border-0"
                  title="Compare Dashboard"
                />
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
