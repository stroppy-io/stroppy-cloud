import { useState, useEffect, useMemo, useRef } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, getPresets } from "@/api/client";
import type {
  RunConfig,
  DatabaseKind,
  Provider,
  PresetsResponse,
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
import { Badge } from "@/components/ui/badge";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  Eye,
  Check,
  AlertCircle,
  ChevronDown,
  ChevronRight,
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
const WORKLOADS = ["simple", "tpcb", "tpcc"];

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

const DB_META: Record<DatabaseKind, { icon: typeof Database; color: string; accent: string; label: string }> = {
  postgres: { icon: Database, color: "text-blue-400", accent: "border-blue-500/40 bg-blue-500/[0.06]", label: "PostgreSQL" },
  mysql:    { icon: Server,   color: "text-orange-400", accent: "border-orange-500/40 bg-orange-500/[0.06]", label: "MySQL" },
  picodata: { icon: Cpu,      color: "text-emerald-400", accent: "border-emerald-500/40 bg-emerald-500/[0.06]", label: "Picodata" },
};

const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud,     label: "Yandex Cloud" },
};

const WORKLOAD_DESC: Record<string, string> = {
  simple: "Basic key-value insert/select",
  tpcb: "TPC-B banking benchmark",
  tpcc: "TPC-C order processing benchmark",
};

// --- Phase header ---

function PhaseHeader({ num, title, subtitle }: { num: number; title: string; subtitle?: string }) {
  return (
    <div className="flex items-center gap-3 mb-4">
      <div className="w-7 h-7 border border-zinc-700 bg-zinc-900 flex items-center justify-center text-[11px] font-mono text-zinc-400 shrink-0">
        {num}
      </div>
      <div>
        <h2 className="text-[13px] font-semibold uppercase tracking-[0.12em] text-zinc-300">{title}</h2>
        {subtitle && <p className="text-[11px] text-zinc-600 mt-0.5">{subtitle}</p>}
      </div>
    </div>
  );
}

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
  const [workload, setWorkload] = useState("simple");
  const [duration, setDuration] = useState("5m");
  const [workers, setWorkers] = useState(4);
  const [cidr, setCidr] = useState("10.0.0.0/24");
  const [showPackages, setShowPackages] = useState(false);
  const [customPackagesJSON, setCustomPackagesJSON] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [dryRunResult, setDryRunResult] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { getPresets().then(setPresets).catch(() => {}); }, []);
  useEffect(() => { setVersion(DB_VERSIONS[kind][0]); setPreset(PRESET_NAMES[kind][0]); }, [kind]);

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
      network: { cidr },
      machines: [],
      database: { kind, version, ...topo } as RunConfig["database"],
      monitor: {},
      stroppy: { version: "3.1.0", workload, duration, workers },
    };
    if (customPackagesJSON.trim()) {
      try { cfg.packages = JSON.parse(customPackagesJSON); } catch { /* ignore */ }
    }
    return cfg;
  }, [kind, preset, provider, version, workload, duration, workers, cidr, presets, customPackagesJSON]);

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
    if (!config.network.cidr.trim()) { setError("Network CIDR is required"); return; }
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
  const DbIcon = dbMeta.icon;

  return (
    <div className="flex flex-col h-full">
      {/* Sticky launch bar */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-3 flex items-center justify-between z-10">
        <div className="flex items-center gap-3">
          <div className={`flex items-center gap-2 px-2.5 py-1 border ${dbMeta.accent}`}>
            <DbIcon className={`h-3.5 w-3.5 ${dbMeta.color}`} />
            <span className={`text-xs font-mono font-medium ${dbMeta.color}`}>{dbMeta.label}</span>
            <span className="text-[10px] text-zinc-600 font-mono">v{version}</span>
          </div>
          <span className="text-zinc-700 text-xs">/</span>
          <span className="text-xs font-mono text-zinc-500">{preset}</span>
          <span className="text-zinc-700 text-xs">/</span>
          <span className="text-xs font-mono text-zinc-500">{workload}</span>
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
            {submitting ? "Launching..." : "Launch Run"}
          </Button>
        </div>
      </div>

      {/* Feedback messages */}
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

      {/* Main content */}
      <div className="flex-1 min-h-0 overflow-auto">
        <div className="grid grid-cols-[1fr_380px] h-full">
          {/* Left: Form */}
          <div className="p-5 space-y-8 border-r border-zinc-800/50 overflow-auto">

            {/* Phase 1: Target */}
            <section>
              <PhaseHeader num={1} title="Target" subtitle="Database engine, version, and deployment target" />

              <div className="grid grid-cols-3 gap-2 mb-4">
                {DB_KINDS.map((k) => {
                  const meta = DB_META[k];
                  const Icon = meta.icon;
                  const active = kind === k;
                  return (
                    <button
                      type="button"
                      key={k}
                      onClick={() => setKind(k)}
                      className={`flex items-center gap-2.5 border p-3 transition-all cursor-pointer ${
                        active
                          ? `${meta.accent} shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]`
                          : "border-zinc-800/60 bg-transparent hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <Icon className={`h-4 w-4 ${active ? meta.color : "text-zinc-600"}`} />
                      <span className={`text-sm font-mono font-medium ${active ? meta.color : "text-zinc-500"}`}>
                        {meta.label}
                      </span>
                    </button>
                  );
                })}
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
                  <Select value={version} onValueChange={setVersion}>
                    <SelectTrigger className="h-9 font-mono text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {DB_VERSIONS[kind].map((v) => (
                        <SelectItem key={v} value={v}>{dbMeta.label} {v}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Provider</Label>
                  <div className="grid grid-cols-2 gap-1.5">
                    {PROVIDERS.map((p) => {
                      const pm = PROVIDER_META[p];
                      const PIcon = pm.icon;
                      const active = provider === p;
                      return (
                        <button
                          type="button"
                          key={p}
                          onClick={() => setProvider(p)}
                          className={`flex items-center gap-1.5 border px-3 py-2 text-xs font-mono transition-colors cursor-pointer ${
                            active
                              ? "border-primary/40 text-primary bg-primary/[0.06]"
                              : "border-zinc-800 text-zinc-500 hover:text-zinc-300 hover:border-zinc-700"
                          }`}
                        >
                          <PIcon className="h-3 w-3" />
                          {pm.label}
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>
            </section>

            {/* Phase 2: Topology */}
            <section>
              <PhaseHeader num={2} title="Topology" subtitle="Cluster architecture and node layout" />

              <div className="grid grid-cols-3 gap-3">
                {PRESET_NAMES[kind].map((p) => {
                  const active = preset === p;
                  return (
                    <button
                      type="button"
                      key={p}
                      onClick={() => setPreset(p)}
                      className={`border p-4 text-left transition-all cursor-pointer ${
                        active
                          ? `${dbMeta.accent} shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]`
                          : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <div className="flex items-center justify-between mb-3">
                        <span className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? dbMeta.color : "text-zinc-400"}`}>
                          {p}
                        </span>
                        {active && (
                          <div className={`w-1.5 h-1.5 rounded-full ${
                            kind === "postgres" ? "bg-blue-400" : kind === "mysql" ? "bg-orange-400" : "bg-emerald-400"
                          }`} />
                        )}
                      </div>
                      <TopologyDiagram kind={kind} preset={p} />
                    </button>
                  );
                })}
              </div>
            </section>

            {/* Phase 3: Workload */}
            <section>
              <PhaseHeader num={3} title="Workload" subtitle="Stroppy benchmark configuration" />

              <div className="grid grid-cols-3 gap-2 mb-4">
                {WORKLOADS.map((w) => {
                  const active = workload === w;
                  return (
                    <button
                      type="button"
                      key={w}
                      onClick={() => setWorkload(w)}
                      className={`border p-3 text-left transition-all cursor-pointer ${
                        active
                          ? "border-primary/40 bg-primary/[0.06]"
                          : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <div className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? "text-primary" : "text-zinc-400"}`}>
                        {w}
                      </div>
                      <div className="text-[10px] text-zinc-600 mt-1">{WORKLOAD_DESC[w]}</div>
                    </button>
                  );
                })}
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Duration</Label>
                  <Input
                    value={duration}
                    onChange={(e) => setDuration(e.target.value)}
                    placeholder="5m"
                    className="font-mono text-xs h-9"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">
                    Workers
                    <span className="ml-2 text-primary tabular-nums">{workers}</span>
                  </Label>
                  <div className="pt-2">
                    <input
                      type="range"
                      min={1}
                      max={64}
                      value={workers}
                      onChange={(e) => setWorkers(Number(e.target.value))}
                      className="w-full h-1 bg-zinc-800 appearance-none cursor-pointer accent-primary"
                    />
                    <div className="flex justify-between text-[9px] text-zinc-700 font-mono mt-1">
                      <span>1</span>
                      <span>16</span>
                      <span>32</span>
                      <span>64</span>
                    </div>
                  </div>
                </div>
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Network CIDR</Label>
                  <Input
                    value={cidr}
                    onChange={(e) => setCidr(e.target.value)}
                    placeholder="10.0.0.0/24"
                    className="font-mono text-xs h-9"
                  />
                </div>
              </div>
            </section>

            {/* Phase 4: Advanced (collapsible) */}
            <section>
              <button
                type="button"
                className="flex items-center gap-3 mb-4 group cursor-pointer"
                onClick={() => setShowPackages(!showPackages)}
              >
                <div className="w-7 h-7 border border-zinc-800 bg-zinc-900/50 flex items-center justify-center text-[11px] font-mono text-zinc-600 shrink-0">
                  4
                </div>
                <div className="flex items-center gap-2">
                  <h2 className="text-[13px] font-semibold uppercase tracking-[0.12em] text-zinc-500 group-hover:text-zinc-300 transition-colors">
                    Packages
                  </h2>
                  <Badge variant="secondary" className="text-[9px]">optional</Badge>
                  {showPackages ? (
                    <ChevronDown className="h-3 w-3 text-zinc-600" />
                  ) : (
                    <ChevronRight className="h-3 w-3 text-zinc-600" />
                  )}
                </div>
              </button>

              {showPackages && (
                <div className="ml-10">
                  <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-1.5 block">
                    PackageSet JSON
                  </Label>
                  <textarea
                    className="w-full h-28 bg-[#050505] border border-zinc-800 p-3 font-mono text-xs text-foreground resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                    value={customPackagesJSON}
                    onChange={(e) => setCustomPackagesJSON(e.target.value)}
                    placeholder='{"apt": ["custom-pkg"], "pre_install_apt": ["apt-get update"]}'
                    spellCheck={false}
                  />
                </div>
              )}
            </section>
          </div>

          {/* Right: Live config */}
          <div className="flex flex-col bg-[#050505] overflow-hidden">
            <div className="shrink-0 flex items-center justify-between px-4 py-2.5 border-b border-zinc-800/50">
              <div className="flex items-center gap-2">
                <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">
                  {dryRunResult ? "Dry Run Output" : "Config"}
                </span>
                <span className="text-[10px] font-mono text-zinc-700 tabular-nums">
                  {runIDRef.current}
                </span>
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
                  className="p-1.5 text-zinc-600 hover:text-zinc-300 transition-colors cursor-pointer"
                  title="Copy to clipboard"
                >
                  {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
                </button>
              </div>
            </div>
            <pre className="flex-1 p-4 text-[11px] font-mono leading-[1.6] text-zinc-400 overflow-auto selection:bg-primary/20">
              {dryRunResult ? JSON.stringify(dryRunResult, null, 2) : configJSON}
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}
