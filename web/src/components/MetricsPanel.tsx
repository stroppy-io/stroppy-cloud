import { useEffect, useState } from "react";
import { getRunMetrics } from "@/api/client";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RefreshCw, TrendingUp, TrendingDown, Minus } from "lucide-react";

interface MetricSummary {
  key: string;
  name: string;
  unit: string;
  avg: number;
  min: number;
  max: number;
  last: number;
}

interface RunMetricsResponse {
  run_id: string;
  range: { Start: string; End: string };
  metrics: MetricSummary[];
}

interface MetricsPanelProps {
  runID: string;
  startedAt?: string;  // ISO from snapshot
  finishedAt?: string; // ISO from snapshot
}

function formatMetricValue(value: number, unit: string): string {
  if (value === 0) return "--";
  if (unit === "%" || unit === "s") return value.toFixed(2);
  if (unit === "ops/s" || unit === "errors/s") return value.toFixed(1);
  if (unit === "bytes/s") {
    if (value > 1e9) return `${(value / 1e9).toFixed(1)} GB/s`;
    if (value > 1e6) return `${(value / 1e6).toFixed(1)} MB/s`;
    if (value > 1e3) return `${(value / 1e3).toFixed(1)} KB/s`;
    return `${value.toFixed(0)} B/s`;
  }
  if (value > 1e6) return `${(value / 1e6).toFixed(1)}M`;
  if (value > 1e3) return `${(value / 1e3).toFixed(1)}K`;
  return value.toFixed(1);
}

function metricIcon(key: string, value: number) {
  if (value === 0) return <Minus className="h-3 w-3 text-zinc-600" />;
  const goodWhenHigh = ["db_qps", "stroppy_ops"];
  if (goodWhenHigh.includes(key)) {
    return <TrendingUp className="h-3 w-3 text-emerald-500" />;
  }
  return <TrendingDown className="h-3 w-3 text-zinc-400" />;
}

const metricCategories: Record<string, string[]> = {
  "Database": ["db_qps", "db_latency_p99", "db_connections", "db_repl_lag"],
  "System": ["cpu_usage", "memory_usage", "disk_read", "disk_write", "net_rx", "net_tx"],
  "Stroppy": ["stroppy_vus", "stroppy_ops", "stroppy_iter_p99", "stroppy_query_rate", "stroppy_latency_p99", "stroppy_errors"],
};

export function MetricsPanel({ runID, startedAt, finishedAt }: MetricsPanelProps) {
  const [metrics, setMetrics] = useState<MetricSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [unavailable, setUnavailable] = useState(false);

  // Use run timestamps if available, otherwise default to last 2 hours.
  const toLocalInput = (d: Date) => {
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  const defaultStart = startedAt
    ? toLocalInput(new Date(new Date(startedAt).getTime() - 60000)) // 1min before run start
    : toLocalInput(new Date(Date.now() - 7200000));
  const defaultEnd = finishedAt && finishedAt !== "0001-01-01T00:00:00Z"
    ? toLocalInput(new Date(new Date(finishedAt).getTime() + 60000)) // 1min after run end
    : toLocalInput(new Date());

  const [start, setStart] = useState(defaultStart);
  const [end, setEnd] = useState(defaultEnd);

  const fetchMetrics = async () => {
    setLoading(true);
    setError(null);
    try {
      // datetime-local values are local time — convert to ISO/UTC for the API.
      const startISO = new Date(start).toISOString();
      const endISO = new Date(end).toISOString();
      const data = (await getRunMetrics(runID, startISO, endISO)) as unknown as RunMetricsResponse;
      setMetrics(data.metrics || []);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to fetch metrics";
      if (msg.includes("503")) {
        setUnavailable(true);
      } else {
        setError(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMetrics();
  }, [runID]);

  const metricsByKey = Object.fromEntries(metrics.map((m) => [m.key, m]));

  if (unavailable) {
    return (
      <div className="text-xs text-zinc-600 p-4">
        Metrics collection not configured. Enable VictoriaMetrics in server settings.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Time range controls */}
      <div className="flex items-end gap-3 p-3 border border-border/50 bg-[#060606]">
        <div className="space-y-1">
          <Label className="text-[10px] uppercase tracking-widest text-zinc-500">From</Label>
          <Input
            type="datetime-local"
            value={start}
            onChange={(e) => setStart(e.target.value)}
            className="h-8 text-xs font-mono bg-transparent border-zinc-800"
          />
        </div>
        <div className="space-y-1">
          <Label className="text-[10px] uppercase tracking-widest text-zinc-500">To</Label>
          <Input
            type="datetime-local"
            value={end}
            onChange={(e) => setEnd(e.target.value)}
            className="h-8 text-xs font-mono bg-transparent border-zinc-800"
          />
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={fetchMetrics}
          disabled={loading}
          className="h-8"
        >
          <RefreshCw className={`h-3 w-3 ${loading ? "animate-spin" : ""}`} />
          Query
        </Button>
      </div>

      {error && (
        <div className="text-xs text-destructive p-2 border border-destructive/30 font-mono">
          {error}
        </div>
      )}

      {/* Metric categories */}
      {Object.entries(metricCategories).map(([category, keys]) => {
        const categoryMetrics = keys
          .map((k) => metricsByKey[k])
          .filter(Boolean);

        return (
          <Card key={category} className="border-zinc-800/60 bg-[#080808]">
            <CardHeader className="pb-2 pt-3 px-4">
              <CardTitle className="text-[11px] uppercase tracking-[0.2em] text-zinc-500 font-normal">
                {category}
              </CardTitle>
            </CardHeader>
            <CardContent className="px-4 pb-3">
              {categoryMetrics.length === 0 ? (
                <p className="text-xs text-zinc-600 font-mono">No data</p>
              ) : (
                <div className="grid grid-cols-2 gap-x-8 gap-y-1">
                  {categoryMetrics.map((m) => (
                    <div
                      key={m.key}
                      className="flex items-center justify-between py-1.5 border-b border-zinc-900 last:border-0"
                    >
                      <div className="flex items-center gap-2">
                        {metricIcon(m.key, m.avg)}
                        <span className="text-xs text-zinc-400">{m.name}</span>
                      </div>
                      <div className="flex items-baseline gap-3 font-mono">
                        <span className="text-xs text-zinc-300">
                          {formatMetricValue(m.avg, m.unit)}
                        </span>
                        <span className="text-[10px] text-zinc-600">
                          {m.unit}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        );
      })}

      {/* Raw data table */}
      {metrics.length > 0 && (
        <details className="group" open={true}>
          <summary className="text-[10px] uppercase tracking-widest text-zinc-600 cursor-pointer hover:text-zinc-400 transition-colors py-2">
            Raw metric values ({metrics.length})
          </summary>
          <div className="mt-2 overflow-auto">
            <table className="w-full text-xs font-mono">
              <thead>
                <tr className="text-[10px] text-zinc-600 uppercase tracking-wider border-b border-zinc-800">
                  <th className="text-left py-1.5 pr-4">Metric</th>
                  <th className="text-right py-1.5 px-3">Avg</th>
                  <th className="text-right py-1.5 px-3">Min</th>
                  <th className="text-right py-1.5 px-3">Max</th>
                  <th className="text-right py-1.5 pl-3">Last</th>
                </tr>
              </thead>
              <tbody>
                {metrics.map((m) => (
                  <tr key={m.key} className="border-b border-zinc-900/50">
                    <td className="py-1.5 pr-4 text-zinc-400">{m.key}</td>
                    <td className="text-right py-1.5 px-3 text-zinc-300">
                      {formatMetricValue(m.avg, m.unit)}
                    </td>
                    <td className="text-right py-1.5 px-3 text-zinc-500">
                      {formatMetricValue(m.min, m.unit)}
                    </td>
                    <td className="text-right py-1.5 px-3 text-zinc-500">
                      {formatMetricValue(m.max, m.unit)}
                    </td>
                    <td className="text-right py-1.5 pl-3 text-zinc-300">
                      {formatMetricValue(m.last, m.unit)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      )}
    </div>
  );
}
