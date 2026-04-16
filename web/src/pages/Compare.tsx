import { useState, useEffect } from "react";
import { useSearchParams, useNavigate, Link } from "react-router-dom";
import { compareRuns, getGrafanaSettings } from "@/api/client";
import type { ComparisonResponse, GrafanaSettings } from "@/api/types";
import { MetricsDiff } from "@/components/MetricsDiff";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { AlertCircle, GitCompare, ArrowLeft, ExternalLink, Loader2, Cpu } from "lucide-react";
import type { RunConfig, MachineSpec } from "@/api/types";

export function Compare() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const runA = searchParams.get("a") || "";
  const runB = searchParams.get("b") || "";

  const [inputA, setInputA] = useState(runA);
  const [inputB, setInputB] = useState(runB);
  const [result, setResult] = useState<ComparisonResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);

  useEffect(() => {
    getGrafanaSettings().then(setGrafana).catch(() => setGrafana(null));
  }, []);

  // Sync inputs when URL params change
  useEffect(() => {
    setInputA(runA);
    setInputB(runB);
  }, [runA, runB]);

  useEffect(() => {
    if (!runA || !runB) return;

    setLoading(true);
    setError(null);
    setResult(null);

    compareRuns(runA, runB)
      .then(setResult)
      .catch((err) => setError(err instanceof Error ? err.message : "Comparison failed"))
      .finally(() => setLoading(false));
  }, [runA, runB]);

  function handleSubmit() {
    const a = inputA.trim();
    const b = inputB.trim();
    if (!a || !b) return;
    navigate(`/compare?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`);
  }

  if (!runA || !runB) {
    return (
      <div className="p-5 space-y-4">
        <div className="flex items-center gap-3">
          <GitCompare className="h-4 w-4 text-primary" />
          <h1 className="text-base font-semibold font-mono">Compare Runs</h1>
        </div>

        <Card className="max-w-lg">
          <CardContent className="pt-6 space-y-4">
            <p className="text-xs text-zinc-500 font-mono">
              Enter two run IDs or select them from the{" "}
              <Link to="/" className="text-primary hover:underline">runs table</Link>.
            </p>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-[11px] font-mono text-zinc-500">Run A</Label>
                <Input
                  value={inputA}
                  onChange={(e) => setInputA(e.target.value)}
                  placeholder="run-abc123"
                  className="font-mono text-xs"
                  onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-mono text-zinc-500">Run B</Label>
                <Input
                  value={inputB}
                  onChange={(e) => setInputB(e.target.value)}
                  placeholder="run-def456"
                  className="font-mono text-xs"
                  onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
                />
              </div>
            </div>
            <Button
              onClick={handleSubmit}
              disabled={!inputA.trim() || !inputB.trim()}
              size="sm"
            >
              <GitCompare className="h-3.5 w-3.5" />
              Compare
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="p-5 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <GitCompare className="h-4 w-4 text-primary" />
          <h1 className="text-base font-semibold font-mono">Compare</h1>
        </div>

        {grafana?.embed_enabled && grafana.dashboards?.compare && result && (
          <a
            href={`${grafana.url}/d/${grafana.dashboards.compare}?var-run_a=${runA}&var-run_b=${runB}&from=${new Date(result.start).getTime()}&to=${new Date(result.end).getTime()}&theme=dark`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5 text-[11px] font-mono text-primary hover:underline"
          >
            <ExternalLink className="h-3 w-3" />
            Open in Grafana
          </a>
        )}
      </div>

      {/* Run ID inputs + links */}
      <div className="flex items-center gap-2 flex-wrap">
        <Input
          value={inputA}
          onChange={(e) => setInputA(e.target.value)}
          className="font-mono text-xs w-56 h-8"
          onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
        />
        <span className="text-zinc-600 text-xs font-mono">vs</span>
        <Input
          value={inputB}
          onChange={(e) => setInputB(e.target.value)}
          className="font-mono text-xs w-56 h-8"
          onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
        />
        {(inputA !== runA || inputB !== runB) && (
          <Button size="sm" onClick={handleSubmit} className="h-8">
            <GitCompare className="h-3 w-3" />
            Compare
          </Button>
        )}
        <div className="flex items-center gap-2 ml-2 text-[10px] font-mono">
          <Link to={`/runs/${runA}`} className="text-cyan-400 hover:underline">{runA}</Link>
          <span className="text-zinc-700">vs</span>
          <Link to={`/runs/${runB}`} className="text-amber-400 hover:underline">{runB}</Link>
        </div>
      </div>

      {/* Loading */}
      {loading && (
        <div className="flex items-center gap-2 py-8 justify-center">
          <Loader2 className="h-4 w-4 text-zinc-500 animate-spin" />
          <span className="text-xs text-zinc-500 font-mono">Fetching metrics...</span>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-center gap-2 text-xs p-3 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      {/* Hardware summary */}
      {result && (result.config_a || result.config_b) && (
        <div className="grid grid-cols-2 gap-3">
          <HardwareCard label="Run A" runId={result.run_a} config={result.config_a} color="text-cyan-400" />
          <HardwareCard label="Run B" runId={result.run_b} config={result.config_b} color="text-amber-400" />
        </div>
      )}

      {/* Results */}
      {result && (
        <Card className="border-zinc-800/80 bg-[#080808]">
          <CardHeader className="pb-0">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-mono">Metric Comparison</CardTitle>
              <div className="flex items-center gap-3">
                <Badge variant="success" className="text-[10px]">
                  {result.summary.better} better
                </Badge>
                <Badge variant="destructive" className="text-[10px]">
                  {result.summary.worse} worse
                </Badge>
                <Badge variant="pending" className="text-[10px]">
                  {result.summary.same} same
                </Badge>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0 pt-3">
            {result.metrics.length > 0 ? (
              <MetricsDiff
                rows={result.metrics}
                runA={result.run_a}
                runB={result.run_b}
              />
            ) : (
              <div className="p-4 text-xs text-zinc-600 font-mono">
                No metrics data found for these runs.
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Grafana embed */}
      {grafana?.embed_enabled && grafana.dashboards?.compare && result && (
        <Card className="border-zinc-800/80">
          <CardHeader>
            <CardTitle className="text-sm font-mono">Grafana Dashboard</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <iframe
              src={`${grafana.url}/d/${grafana.dashboards.compare}?var-run_a=${runA}&var-run_b=${runB}&from=${new Date(result.start).getTime()}&to=${new Date(result.end).getTime()}&kiosk&theme=dark`}
              className="w-full h-[600px] border-0"
              title="Compare Dashboard"
              sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
            />
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function fmtMem(mb: number): string {
  return mb >= 1024 ? `${(mb / 1024).toFixed(mb % 1024 ? 1 : 0)} GB` : `${mb} MB`;
}

function dbMachineFromConfig(cfg: RunConfig): MachineSpec | null {
  const db = cfg.database;
  if (db?.postgres?.master) return db.postgres.master;
  if (db?.mysql?.primary) return db.mysql.primary;
  if (db?.picodata?.instances?.[0]) return db.picodata.instances[0];
  if (db?.ydb?.storage) return db.ydb.storage;
  if (cfg.machine_override) return cfg.machine_override;
  return cfg.machines?.find((m) => m.role === "database") ?? null;
}

function HardwareCard({ label, runId, config, color }: { label: string; runId: string; config?: RunConfig; color: string }) {
  if (!config) return null;
  const db = dbMachineFromConfig(config);
  const stroppy = config.stroppy?.machine;
  return (
    <Card className="border-zinc-800/80 bg-[#080808]">
      <CardContent className="py-3 px-4 space-y-2">
        <div className="flex items-center gap-2">
          <Cpu className={`h-3.5 w-3.5 ${color}`} />
          <span className={`text-[11px] font-mono font-medium ${color}`}>{label}</span>
          <span className="text-[10px] font-mono text-zinc-600">{runId}</span>
        </div>
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[10px] font-mono">
          <span className="text-zinc-500">DB</span>
          <span className="text-zinc-300">{config.database?.kind} {config.database?.version}</span>
          {config.platform_id && <>
            <span className="text-zinc-500">Platform</span>
            <span className="text-zinc-300">{config.platform_id}</span>
          </>}
          {db && <>
            <span className="text-zinc-500">DB Machine</span>
            <span className="text-zinc-300">{db.cpus} vCPU / {fmtMem(db.memory_mb)} / {db.disk_gb} GB {db.disk_type || ""}</span>
          </>}
          {stroppy && <>
            <span className="text-zinc-500">Runner</span>
            <span className="text-zinc-300">{stroppy.cpus} vCPU / {fmtMem(stroppy.memory_mb)}</span>
          </>}
          <span className="text-zinc-500">Workload</span>
          <span className="text-zinc-300">{config.stroppy?.script} / {config.stroppy?.vus} VUs / SF {config.stroppy?.scale_factor || 1}</span>
          <span className="text-zinc-500">Duration</span>
          <span className="text-zinc-300">{config.stroppy?.duration}</span>
          {config.provider && <>
            <span className="text-zinc-500">Provider</span>
            <span className="text-zinc-300">{config.provider}</span>
          </>}
        </div>
      </CardContent>
    </Card>
  );
}
