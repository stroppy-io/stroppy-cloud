import { useState, useEffect, useMemo, useRef } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, getPresets, listPackages } from "@/api/client";
import type {
  RunConfig,
  DatabaseKind,
  Provider,
  PresetsResponse,
  Package,
} from "@/api/types";
import { generateRunID } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  Eye,
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Cloud,
  Container,
  Copy,
  Rocket,
} from "lucide-react";

// --- Constants ---

const DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "picodata"];
const PROVIDERS: Provider[] = ["docker", "yandex"];
const WORKLOADS = ["tpcb", "tpcc"];

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["16", "17"],
  mysql: ["8.0", "8.4"],
  picodata: ["25.3"],
};

const PRESET_NAMES: Record<DatabaseKind, string[]> = {
  postgres: ["single", "ha", "scale"],
  mysql: ["single", "replica", "group"],
  picodata: ["single", "cluster", "scale"],
};

import { DB_COLORS } from "@/lib/db-colors";

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres: { icon: Database, label: "PostgreSQL" },
  mysql:    { icon: Server,   label: "MySQL" },
  picodata: { icon: Cpu,      label: "Picodata" },
};

const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud,     label: "Yandex Cloud" },
};

const WORKLOAD_DESC: Record<string, string> = {
  tpcb: "TPC-B banking",
  tpcc: "TPC-C orders",
};

// --- Main ---

export function NewRun() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const [presets, setPresets] = useState<PresetsResponse | null>(null);
  const [kind, setKind] = useState<DatabaseKind>(
    (searchParams.get("kind") as DatabaseKind) || "postgres"
  );
  const [preset, setPreset] = useState(searchParams.get("preset") || "single");
  const [provider, setProvider] = useState<Provider>("docker");
  const [version, setVersion] = useState(DB_VERSIONS[kind][0]);
  const [workload, setWorkload] = useState("tpcc");
  const [duration, setDuration] = useState("5m");
  const [vusScale, setVusScale] = useState("1");
  const [poolSize, setPoolSize] = useState("100");
  const [scaleFactor, setScaleFactor] = useState("1");
  const [packageId, setPackageId] = useState("");
  const [availablePackages, setAvailablePackages] = useState<Package[]>([]);

  const [submitting, setSubmitting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [dryRunResult, setDryRunResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { getPresets().then(setPresets).catch(() => {}); }, []);
  useEffect(() => { setVersion(DB_VERSIONS[kind][0]); setPreset(PRESET_NAMES[kind][0]); }, [kind]);
  useEffect(() => {
    listPackages({ db_kind: kind, db_version: version }).then(setAvailablePackages).catch(() => {});
  }, [kind, version]);

  const runIDRef = useRef(generateRunID());

  const config = useMemo((): RunConfig => {
    const id = runIDRef.current;
    const buildTopology = () => {
      if (presets) {
        const p = kind === "postgres" ? presets.postgres[preset]
          : kind === "mysql" ? presets.mysql[preset]
          : presets.picodata[preset];
        if (p) {
          if (kind === "postgres") return { postgres: p };
          if (kind === "mysql") return { mysql: p };
          if (kind === "picodata") return { picodata: p };
        }
      }
      return {};
    };
    const topo = buildTopology();
    const cfg: RunConfig = {
      id, provider,
      network: { cidr: "10.0.0.0/24" },
      machines: [],
      database: { kind, version, ...topo } as RunConfig["database"],
      monitor: {},
      stroppy: {
        version: "3.1.0",
        workload,
        duration,
        vus_scale: parseFloat(vusScale) || 1,
        pool_size: parseInt(poolSize) || 100,
        scale_factor: parseInt(scaleFactor) || 1,
      },
    };
    if (packageId) {
      cfg.package_id = packageId;
    }
    return cfg;
  }, [kind, preset, provider, version, workload, duration, vusScale, poolSize, scaleFactor, presets, packageId]);

  const configJSON = useMemo(() => JSON.stringify(config, null, 2), [config]);

  async function handleValidate() {
    setValidating(true); setValidationResult(null);
    try {
      await validateRun(config);
      setValidationResult({ ok: true, message: "Configuration is valid" });
    } catch (err) {
      setValidationResult({ ok: false, message: err instanceof Error ? err.message : "Validation failed" });
    }
    setValidating(false);
  }

  async function handleDryRun() {
    setDryRunResult(null);
    try { setDryRunResult(await dryRun(config)); }
    catch (err) { setError(err instanceof Error ? err.message : "Dry run failed"); }
  }

  async function handleSubmit() {
    if (!config.stroppy.duration.trim()) { setError("Duration is required"); return; }
    setSubmitting(true); setError(null);
    try {
      const result = await startRun(config);
      navigate(`/runs/${result.run_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start run");
      setSubmitting(false);
    }
  }

  function handleCopy() {
    navigator.clipboard.writeText(configJSON);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  const dbMeta = DB_META[kind];
  const dbColor = DB_COLORS[kind];
  const DbIcon = dbMeta.icon;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Sticky top bar */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5 flex items-center justify-between z-10">
        <div className="flex items-center gap-3">
          <div className={`flex items-center gap-2 px-2.5 py-1 border ${dbColor.accent}`}>
            <DbIcon className={`h-3.5 w-3.5 ${dbColor.text}`} />
            <span className={`text-xs font-mono font-medium ${dbColor.text}`}>{dbMeta.label}</span>
            <span className="text-[10px] text-zinc-600 font-mono">v{version}</span>
          </div>
          <span className="text-zinc-700 text-xs">/</span>
          <span className="text-xs font-mono text-zinc-500">{preset}</span>
          <span className="text-zinc-700 text-xs">/</span>
          <span className="text-xs font-mono text-zinc-500">{workload.toUpperCase()}</span>
          <span className="text-zinc-700 text-xs">/</span>
          <span className="text-xs font-mono text-zinc-600">{PROVIDER_META[provider].label}</span>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={handleValidate} disabled={validating}>
            <Check className="h-3 w-3" />
            {validating ? "..." : "Validate"}
          </Button>
          <Button variant="outline" size="sm" onClick={handleDryRun}>
            <Eye className="h-3 w-3" />
            Dry Run
          </Button>
          <Button size="sm" onClick={handleSubmit} disabled={submitting} className="gap-1.5">
            <Rocket className="h-3.5 w-3.5" />
            {submitting ? "Launching..." : "Launch"}
          </Button>
        </div>
      </div>

      {/* Feedback */}
      {(validationResult || error) && (
        <div className="shrink-0 px-5 py-2 border-b border-zinc-800/50">
          {validationResult && (
            <div className={`flex items-center gap-2 text-xs p-2 border font-mono ${
              validationResult.ok ? "border-success/30 text-success" : "border-destructive/30 text-destructive"
            }`}>
              {validationResult.ok ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
              {validationResult.message}
            </div>
          )}
          {error && (
            <div className="flex items-center gap-2 text-xs p-2 border border-destructive/30 text-destructive font-mono">
              <AlertCircle className="h-3 w-3" />
              {error}
            </div>
          )}
        </div>
      )}

      {/* Main: form left | JSON right */}
      <div className="flex-1 min-h-0 flex overflow-hidden">
        {/* Left — form */}
        <div className="flex-1 min-w-0 overflow-y-auto p-5">
          <div className="grid grid-cols-2 gap-x-6 gap-y-5">

            {/* ── Provider ── */}
            <div className="col-span-2">
              <h2 className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-3">Provider</h2>
              <div className="grid grid-cols-2 gap-2">
                {PROVIDERS.map((p) => {
                  const pm = PROVIDER_META[p];
                  const PIcon = pm.icon;
                  const active = provider === p;
                  return (
                    <button
                      type="button"
                      key={p}
                      onClick={() => setProvider(p)}
                      className={`flex items-center gap-2.5 border p-2.5 transition-all cursor-pointer ${
                        active
                          ? "border-primary/40 text-primary bg-primary/[0.06] shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                          : "border-zinc-800/60 bg-transparent hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <PIcon className={`h-4 w-4 ${active ? "text-primary" : "text-zinc-600"}`} />
                      <span className={`text-sm font-mono font-medium ${active ? "text-primary" : "text-zinc-500"}`}>
                        {pm.label}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* ── Database ── */}
            <div className="col-span-2">
              <h2 className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-3">Database</h2>
              <div className="grid grid-cols-3 gap-2">
                {DB_KINDS.map((k) => {
                  const meta = DB_META[k];
                  const kColor = DB_COLORS[k];
                  const Icon = meta.icon;
                  const active = kind === k;
                  return (
                    <button
                      type="button"
                      key={k}
                      onClick={() => setKind(k)}
                      className={`flex items-center gap-2.5 border p-2.5 transition-all cursor-pointer ${
                        active
                          ? `${kColor.accent} shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]`
                          : "border-zinc-800/60 bg-transparent hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <Icon className={`h-4 w-4 ${active ? kColor.text : "text-zinc-600"}`} />
                      <span className={`text-sm font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>
                        {meta.label}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Version + Package */}
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
              <Select value={version} onValueChange={setVersion}>
                <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {DB_VERSIONS[kind].map((v) => (
                    <SelectItem key={v} value={v}>{dbMeta.label} {v}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Package</Label>
              <Select value={packageId || "__default__"} onValueChange={(v) => setPackageId(v === "__default__" ? "" : v)}>
                <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">Default</SelectItem>
                  {availablePackages.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}{p.has_deb ? " [.deb]" : ""}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <span className="text-[9px] text-zinc-700 font-mono">
                <a href="/packages" className="text-zinc-500 hover:text-zinc-300">manage</a>
              </span>
            </div>

            {/* ── Topology ── */}
            <div className="col-span-2">
              <h2 className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-3">Topology</h2>
              <div className="grid grid-cols-3 gap-3">
                {PRESET_NAMES[kind].map((p) => {
                  const active = preset === p;
                  return (
                    <button
                      type="button"
                      key={p}
                      onClick={() => setPreset(p)}
                      className={`border p-3 text-left transition-all cursor-pointer ${
                        active
                          ? `${dbColor.accent} shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]`
                          : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <div className="flex items-center justify-between mb-2">
                        <span className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? dbColor.text : "text-zinc-400"}`}>
                          {p}
                        </span>
                        {active && <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: dbColor.hex }} />}
                      </div>
                      <TopologyDiagram kind={kind} preset={p} />
                    </button>
                  );
                })}
              </div>
            </div>

            {/* ── Workload ── */}
            <div className="col-span-2">
              <h2 className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-3">Workload</h2>
              <div className="grid grid-cols-2 gap-2">
                {WORKLOADS.map((w) => {
                  const active = workload === w;
                  return (
                    <button
                      type="button"
                      key={w}
                      onClick={() => setWorkload(w)}
                      className={`border p-2.5 text-left transition-all cursor-pointer ${
                        active
                          ? "border-primary/40 bg-primary/[0.06]"
                          : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <div className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? "text-primary" : "text-zinc-400"}`}>
                        {w}
                      </div>
                      <div className="text-[10px] text-zinc-600 mt-0.5">{WORKLOAD_DESC[w]}</div>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Duration + Scale Factor */}
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Duration</Label>
              <Input value={duration} onChange={(e) => setDuration(e.target.value)} placeholder="5m" className="font-mono text-xs h-8" />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Scale Factor</Label>
              <Input value={scaleFactor} onChange={(e) => setScaleFactor(e.target.value)} placeholder="1" className="font-mono text-xs h-8" />
              <span className="text-[9px] text-zinc-700 font-mono">TPC-C warehouses</span>
            </div>

            {/* VUS + Pool */}
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">VUS Scale</Label>
              <Input value={vusScale} onChange={(e) => setVusScale(e.target.value)} placeholder="1" className="font-mono text-xs h-8" />
              <span className="text-[9px] text-zinc-700 font-mono">1 = ~99 VUs for TPC-C</span>
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Pool Size</Label>
              <Input value={poolSize} onChange={(e) => setPoolSize(e.target.value)} placeholder="100" className="font-mono text-xs h-8" />
              <span className="text-[9px] text-zinc-700 font-mono">DB connections</span>
            </div>

          </div>
        </div>

        {/* Right — live JSON config */}
        <div className="w-80 shrink-0 flex flex-col bg-[#050505] border-l border-zinc-800/50 overflow-hidden">
          <div className="shrink-0 flex items-center justify-between px-4 py-2 border-b border-zinc-800/50">
            <div className="flex items-center gap-2">
              <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">
                {dryRunResult ? "Dry Run" : "Config"}
              </span>
              <span className="text-[10px] font-mono text-zinc-700 tabular-nums">{runIDRef.current}</span>
            </div>
            <div className="flex items-center gap-1">
              {dryRunResult != null && (
                <button
                  type="button"
                  onClick={() => setDryRunResult(null)}
                  className="px-2 py-0.5 text-[10px] font-mono text-zinc-500 hover:text-zinc-300 border border-zinc-800 hover:border-zinc-700 transition-colors cursor-pointer"
                >
                  Config
                </button>
              )}
              <button
                type="button"
                onClick={handleCopy}
                className="p-1 text-zinc-600 hover:text-zinc-300 transition-colors cursor-pointer"
                title="Copy"
              >
                {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
              </button>
            </div>
          </div>
          <pre className="flex-1 p-3 text-[11px] font-mono leading-[1.6] text-zinc-400 overflow-auto selection:bg-primary/20">
            {dryRunResult ? JSON.stringify(dryRunResult, null, 2) : configJSON}
          </pre>
        </div>
      </div>
    </div>
  );
}
