import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, listPresets, listPackages, probeScript, getStroppyVersions, getSettings } from "@/api/client";
import {
  ALL_DB_KINDS,
  type RunConfig,
  type DatabaseKind,
  type Provider,
  type Preset,
  type Package,
  type ProbeResponse,
  type TenantQuotas,
} from "@/api/types";
import { generateRunID } from "@/lib/utils";
import { Button } from "@/components/ui/button";
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
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Cloud,
  Container,
  Copy,
  Rocket,
  ChevronRight,
  ChevronLeft,
  ChevronDown,
  Loader2,
  Pencil,
} from "lucide-react";

import { DB_COLORS } from "@/lib/db-colors";
import { NumericSlider, DurationSlider, SliderField, CPU_STEPS, ramSteps, DiskTypeSelect, PlatformSelect, cpuStepsForPlatform, platformLimits, closestStep, diskStepsForType } from "@/components/ui/sliders";

// --- Constants ---

const DB_KINDS = ALL_DB_KINDS;
const PROVIDERS: Provider[] = ["docker", "yandex"];
const SCRIPTS: { id: string; label: string; desc: string; dbs: DatabaseKind[] }[] = [
  { id: "tpcc/procs", label: "TPC-C Procs", desc: "Stored procedures", dbs: ["postgres", "mysql"] },
  { id: "tpcc/tx", label: "TPC-C Tx", desc: "Raw transactions", dbs: ["postgres", "mysql", "picodata", "ydb"] },
  { id: "tpcb/procs", label: "TPC-B Procs", desc: "Stored procedures", dbs: ["postgres", "mysql"] },
  { id: "tpcb/tx", label: "TPC-B Tx", desc: "Raw transactions", dbs: ["postgres", "mysql", "picodata", "ydb"] },
];

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["17", "16", "15"],
  mysql: ["8.4", "8.0"],
  picodata: ["25.3"],
  ydb: ["25.2", "25.1", "24.4", "24.3"],
};

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres: { icon: Database, label: "PostgreSQL" },
  mysql:    { icon: Server,   label: "MySQL" },
  picodata: { icon: Cpu,      label: "Picodata" },
  ydb:      { icon: Database, label: "YDB" },
};

const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud,     label: "Yandex Cloud" },
};


const STEPS = [
  { key: "infra", label: "Infrastructure" },
  { key: "database", label: "Database" },
  { key: "stroppy", label: "Workload" },
  { key: "review", label: "Review & Launch" },
];

// --- Main ---

export function NewRun() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  // Check for rerun config from sessionStorage (set by RunDetail's Rerun button).
  const rerunConfig = useMemo<RunConfig | null>(() => {
    try {
      const raw = sessionStorage.getItem("rerun_config");
      if (raw) {
        sessionStorage.removeItem("rerun_config");
        return JSON.parse(raw) as RunConfig;
      }
    } catch { /* ignore */ }
    return null;
  }, []);

  const rc = rerunConfig; // shorthand
  const rcS = rc?.stroppy;
  const rcOv = rc?.machine_override;

  const [step, setStep] = useState(0);

  const [allPresets, setAllPresets] = useState<Preset[]>([]);
  const [kind, setKind] = useState<DatabaseKind>(
    rc?.database?.kind as DatabaseKind || (searchParams.get("kind") as DatabaseKind) || "postgres"
  );
  const [selectedPresetId, setSelectedPresetId] = useState(rc?.preset_id || searchParams.get("preset_id") || "");
  const [provider, setProvider] = useState<Provider>(rc?.provider || "docker");
  const [platformId, setPlatformId] = useState(rc?.platform_id || "standard-v3");
  const [version, setVersion] = useState(rc?.database?.version || DB_VERSIONS[kind][0]);
  const [script, setScript] = useState(rcS?.script || rcS?.workload || "tpcc/procs");
  const [duration, setDuration] = useState(rcS?.duration || "5m");
  const [vus, setVus] = useState(rcS?.vus || rcS?.vus_scale || 10);
  const [poolSize, setPoolSize] = useState(rcS?.pool_size || 100);
  const [scaleFactor, setScaleFactor] = useState(rcS?.scale_factor || 1);
  const [stroppyVersion, setStroppyVersion] = useState(rcS?.version || "4.1.0");
  const [stroppyVersions, setStroppyVersions] = useState<string[]>([rcS?.version || "4.1.0"]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const versionsLoaded = useRef(false);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>(rcS?.steps || []);
  const [noSteps, setNoSteps] = useState<string[]>(rcS?.no_steps || []);
  const [dbCpus, setDbCpus] = useState(rcOv?.cpus || 2);
  const [dbMemory, setDbMemory] = useState(rcOv?.memory_mb || 4096);
  const [dbDisk, setDbDisk] = useState(rcOv?.disk_gb || 25);
  const [dbDiskType, setDbDiskType] = useState(rcOv?.disk_type || "network-ssd");
  const [packageId, setPackageId] = useState(rc?.package_id || "");
  const [availablePackages, setAvailablePackages] = useState<Package[]>([]);
  const [quotas, setQuotas] = useState<TenantQuotas>({});

  const allowedKinds = useMemo(() =>
    quotas.allowed_db_kinds?.length ? DB_KINDS.filter((k) => quotas.allowed_db_kinds!.includes(k)) : DB_KINDS,
    [quotas]);
  const allowedProviders = useMemo(() =>
    quotas.allowed_providers?.length ? PROVIDERS.filter((p) => quotas.allowed_providers!.includes(p)) : PROVIDERS,
    [quotas]);

  const [submitting, setSubmitting] = useState(false);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [dryRunResult, setDryRunResult] = useState<any>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [resolvedConfig, setResolvedConfig] = useState<RunConfig | null>(null);
  const [dbConfigDraft, setDbConfigDraft] = useState<string | null>(null);
  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(null);
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { listPresets().then(setAllPresets).catch(() => {}); }, []);
  useEffect(() => { getSettings().then((s) => setQuotas(s.quotas || {})).catch(() => {}); }, []);
  useEffect(() => {
    const matching = allPresets.filter((p) => p.db_kind === kind);
    if (matching.length > 0 && !matching.find((p) => p.id === selectedPresetId)) {
      setSelectedPresetId(matching[0].id);
    }
    setVersion(DB_VERSIONS[kind][0]);
    // Reset script if incompatible with new db kind.
    const compatible = SCRIPTS.filter((s) => s.dbs.includes(kind));
    if (!compatible.find((s) => s.id === script)) {
      setScript(compatible[0]?.id || "tpcb/tx");
    }
  }, [kind, allPresets]);
  useEffect(() => {
    listPackages({ db_kind: kind, db_version: version }).then(setAvailablePackages).catch(() => {});
  }, [kind, version]);

  const presetsForKind = useMemo(
    () => allPresets.filter((p) => p.db_kind === kind),
    [allPresets, kind]
  );

  const selectedPreset = useMemo(
    () => presetsForKind.find((p) => p.id === selectedPresetId),
    [presetsForKind, selectedPresetId]
  );

  const runIDRef = useRef(generateRunID());

  const config = useMemo((): RunConfig => {
    const id = runIDRef.current;
    const cfg: RunConfig = {
      id, provider,
      network: { cidr: "10.0.0.0/24" },
      machines: [],
      database: { kind, version },
      monitor: {},
      stroppy: {
        version: stroppyVersion,
        script,
        duration,
        vus,
        pool_size: poolSize,
        scale_factor: scaleFactor,
        ...(selectedSteps.length > 0 ? { steps: selectedSteps } : {}),
        ...(noSteps.length > 0 ? { no_steps: noSteps } : {}),
        ...(provider === "yandex" ? (() => { const s = suggestStroppyMachine(vus, poolSize); return { machine: { role: "stroppy" as const, count: 1, cpus: s.cpus, memory_mb: s.memory, disk_gb: s.disk } }; })() : {}),
      },
    };
    if (selectedPresetId) cfg.preset_id = selectedPresetId;
    if (packageId) cfg.package_id = packageId;
    if (provider === "yandex") {
      cfg.platform_id = platformId;
      cfg.machine_override = { role: "database", count: 1, cpus: dbCpus, memory_mb: dbMemory, disk_gb: dbDisk, disk_type: dbDiskType };
    }
    return cfg;
  }, [kind, selectedPresetId, provider, platformId, version, script, duration, vus, poolSize, scaleFactor, packageId, selectedSteps, noSteps, dbCpus, dbMemory, dbDisk, dbDiskType, stroppyVersion]);

  const configJSON = useMemo(() => JSON.stringify(config, null, 2), [config]);

  // Auto-validate on step change
  useEffect(() => {
    if (step < 3) { setValidationResult(null); return; }
    let cancelled = false;
    setDryRunLoading(true);
    setDryRunResult(null);
    setValidationResult(null);
    setError(null);
    (async () => {
      try {
        await validateRun(config);
        if (!cancelled) setValidationResult({ ok: true, message: "Configuration is valid" });
      } catch (err) {
        if (!cancelled) setValidationResult({ ok: false, message: err instanceof Error ? err.message : "Validation failed" });
      }
      try {
        const dr = await dryRun(config);
        if (!cancelled) {
          setDryRunResult(dr);
          const drObj = dr as Record<string, unknown>;
          if (drObj.resolved_config) {
            const rc = drObj.resolved_config as RunConfig;
            setResolvedConfig(rc);
            setDbConfigDraft(JSON.stringify(rc.database, null, 2));
          }
          if (typeof drObj.stroppy_config === "string") {
            setStroppyConfigDraft(drObj.stroppy_config);
            setStroppyConfigPristine(drObj.stroppy_config);
          }
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Dry run failed");
      }
      if (!cancelled) setDryRunLoading(false);
    })();
    return () => { cancelled = true; };
  }, [step, config]);

  const handleSubmit = useCallback(async () => {
    if (!config.stroppy.duration.trim()) { setError("Duration is required"); return; }
    setSubmitting(true); setError(null);
    try {
      // Merge edited database config from review textarea if available.
      const launchConfig = { ...config };
      if (dbConfigDraft && resolvedConfig) {
        try {
          launchConfig.database = JSON.parse(dbConfigDraft);
        } catch {
          setError("Invalid JSON in database config");
          setSubmitting(false);
          return;
        }
      }
      // Only send override when the user actually edited the preview — the server
      // builds the preview with placeholder db host/port (resolved at run time), so
      // sending it back verbatim would ship those placeholders to the stroppy binary.
      if (stroppyConfigDraft && stroppyConfigDraft !== stroppyConfigPristine) {
        try {
          JSON.parse(stroppyConfigDraft); // validate
          launchConfig.stroppy = { ...launchConfig.stroppy, config_override_json: stroppyConfigDraft };
        } catch {
          setError("Invalid JSON in stroppy config");
          setSubmitting(false);
          return;
        }
      }
      const result = await startRun(launchConfig);
      navigate(`/runs/${result.run_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start run");
      setSubmitting(false);
    }
  }, [config, navigate, dbConfigDraft, resolvedConfig, stroppyConfigDraft, stroppyConfigPristine]);

  function handleCopy() {
    navigator.clipboard.writeText(configJSON);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  const dbMeta = DB_META[kind];
  const dbColor = DB_COLORS[kind];
  const DbIcon = dbMeta.icon;
  const ProvIcon = PROVIDER_META[provider].icon;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Step indicator */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <div className="flex items-center gap-1">
          {STEPS.map((s, i) => (
            <div key={s.key} className="flex items-center gap-1">
              {i > 0 && <ChevronRight className="w-3 h-3 text-zinc-700" />}
              <button
                type="button"
                onClick={() => setStep(i)}
                className={`px-3 py-1 text-xs font-mono transition-all cursor-pointer ${
                  i === step
                    ? "text-primary border border-primary/30 bg-primary/[0.06]"
                    : i < step
                      ? "text-zinc-400 hover:text-zinc-300"
                      : "text-zinc-600"
                }`}
              >
                <span className="text-zinc-600 mr-1.5">{i + 1}.</span>
                {s.label}
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Main: step content left | sidebar right */}
      <div className="flex-1 min-h-0 flex overflow-hidden">
        {/* Left — step content */}
        <div className="flex-1 min-w-0 overflow-y-auto p-5">
          {step === 0 && (
            <StepInfra provider={provider} setProvider={setProvider} platformId={platformId} setPlatformId={setPlatformId} providers={allowedProviders} />
          )}
          {step === 1 && (
            <StepDatabase
              kind={kind} setKind={setKind}
              version={version} setVersion={setVersion}
              packageId={packageId} setPackageId={setPackageId}
              availablePackages={availablePackages}
              presetsForKind={presetsForKind}
              selectedPresetId={selectedPresetId} setSelectedPresetId={setSelectedPresetId}
              dbMeta={dbMeta} dbColor={dbColor}
              allowedKinds={allowedKinds}
            />
          )}
          {step === 2 && (
            <StepStroppy
              script={script} setScript={setScript}
              duration={duration} setDuration={setDuration}
              scaleFactor={scaleFactor} setScaleFactor={setScaleFactor}
              vus={vus} setVus={setVus}
              poolSize={poolSize} setPoolSize={setPoolSize}
              dbKind={kind}
              provider={provider}
              platformId={platformId}
              quotas={quotas}
              availableSteps={availableSteps} setAvailableSteps={setAvailableSteps}
              selectedSteps={selectedSteps} setSelectedSteps={setSelectedSteps}
              noSteps={noSteps} setNoSteps={setNoSteps}
              probeData={probeData} setProbeData={setProbeData}
              dbCpus={dbCpus} setDbCpus={setDbCpus}
              dbMemory={dbMemory} setDbMemory={setDbMemory}
              dbDisk={dbDisk} setDbDisk={setDbDisk}
              dbDiskType={dbDiskType} setDbDiskType={setDbDiskType}
              stroppyVersion={stroppyVersion} setStroppyVersion={setStroppyVersion}
              stroppyVersions={stroppyVersions} setStroppyVersions={setStroppyVersions}
              versionsLoading={versionsLoading} setVersionsLoading={setVersionsLoading}
              versionsLoaded={versionsLoaded}
            />
          )}
          {step === 3 && (
            <StepReview
              dryRunResult={dryRunResult}
              dryRunLoading={dryRunLoading}
              validationResult={validationResult}
              error={error}
              submitting={submitting}
              onSubmit={handleSubmit}
              onEdit={(group, key, value) => {
                const n = parseInt(value);
                if (group === "database") {
                  if (key === "cpu_count" && !isNaN(n)) setDbCpus(closestStep(n, cpuStepsForPlatform(platformId)));
                  else if (key === "mem_limit" && !isNaN(n)) setDbMemory(Math.round(n * 100 / 85)); // reverse 85%
                  else if (key === "pdisk_gb" && !isNaN(n)) setDbDisk(n + 2); // reverse disk-2
                } else if (group === "benchmark") {
                  if (key === "VUs" && !isNaN(n)) setVus(n);
                  else if (key === "duration") setDuration(value);
                  else if (key === "pool" && !isNaN(n)) setPoolSize(n);
                  else if (key === "scale" && !isNaN(n)) setScaleFactor(n);
                } else if (group === "infrastructure") {
                  if (key === "platform") setPlatformId(value);
                }
              }}
              dbConfigDraft={dbConfigDraft}
              setDbConfigDraft={setDbConfigDraft}
              stroppyConfigDraft={stroppyConfigDraft}
              setStroppyConfigDraft={setStroppyConfigDraft}
            />
          )}

          {/* Navigation */}
          {step < 3 && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-zinc-800/50">
              <Button variant="outline" size="sm" onClick={() => setStep(Math.max(0, step - 1))} disabled={step === 0}>
                <ChevronLeft className="h-3 w-3" /> Back
              </Button>
              <Button size="sm" onClick={() => setStep(step + 1)} className="gap-1.5">
                Next <ChevronRight className="h-3 w-3" />
              </Button>
            </div>
          )}
        </div>

        {/* Right sidebar */}
        <div className="w-80 shrink-0 flex flex-col bg-[#050505] border-l border-zinc-800/50 overflow-hidden">
          {/* Summary */}
          <div className="shrink-0 px-4 py-3 border-b border-zinc-800/50 space-y-2">
            <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">Setup Summary</div>
            <div className="space-y-1.5">
              <SummaryRow icon={ProvIcon} label="Provider" value={PROVIDER_META[provider].label} />
              {provider === "yandex" && <SummaryRow label="Platform" value={platformLimits(platformId).label} />}
              <SummaryRow icon={DbIcon} label="Database" value={`${dbMeta.label} ${version}`} color={dbColor.text} />
              {selectedPreset && (
                <SummaryRow label="Topology" value={selectedPreset.name} />
              )}
              <SummaryRow label="Script" value={script} />
              <SummaryRow label="Duration" value={duration} />
              <SummaryRow label="VUs" value={String(vus)} />
              <SummaryRow label="Pool" value={String(poolSize)} />
              {scaleFactor > 1 && <SummaryRow label="Scale" value={String(scaleFactor)} />}
            </div>
          </div>

          {/* Config JSON */}
          <div className="shrink-0 flex items-center justify-between px-4 py-2 border-b border-zinc-800/50">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Config</span>
              <span className="text-[9px] font-mono text-zinc-700 tabular-nums">{runIDRef.current}</span>
            </div>
            <button type="button" onClick={handleCopy}
              className="p-1 text-zinc-600 hover:text-zinc-300 transition-colors cursor-pointer" title="Copy">
              {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
            </button>
          </div>
          <pre className="flex-1 p-3 text-[10px] font-mono leading-[1.5] text-zinc-500 overflow-auto selection:bg-primary/20">
            {configJSON}
          </pre>
        </div>
      </div>
    </div>
  );
}

// ─── Summary Row ─────────────────────────────────────────────────

function SummaryRow({ icon: Icon, label, value, color }: {
  icon?: typeof Database;
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div className="flex items-center gap-2 text-[11px] font-mono">
      {Icon && <Icon className={`w-3 h-3 shrink-0 ${color || "text-zinc-600"}`} />}
      {!Icon && <span className="w-3" />}
      <span className="text-zinc-600">{label}</span>
      <span className={`ml-auto ${color || "text-zinc-400"}`}>{value}</span>
    </div>
  );
}

// ─── Step 1: Infrastructure ──────────────────────────────────────

function StepInfra({ provider, setProvider, platformId, setPlatformId, providers }: {
  provider: Provider;
  setProvider: (p: Provider) => void;
  platformId: string;
  setPlatformId: (p: string) => void;
  providers: Provider[];
}) {
  return (
    <div className="space-y-5 max-w-lg">
      <div>
        <h2 className="text-sm font-semibold mb-1">Where to run?</h2>
        <p className="text-xs text-zinc-500">Choose the infrastructure provider for provisioning machines.</p>
      </div>
      <div className="grid grid-cols-2 gap-3">
        {providers.map((p) => {
          const pm = PROVIDER_META[p];
          const PIcon = pm.icon;
          const active = provider === p;
          return (
            <button type="button" key={p} onClick={() => setProvider(p)}
              className={`flex items-center gap-3 border p-4 transition-all cursor-pointer ${
                active
                  ? "border-primary/40 text-primary bg-primary/[0.06]"
                  : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <PIcon className={`h-5 w-5 shrink-0 ${active ? "text-primary" : "text-zinc-600"}`} />
              <div className="text-left">
                <div className={`text-xs font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{pm.label}</div>
                <div className="text-[10px] text-zinc-600">
                  {p === "docker" ? "Local containers" : "Yandex Cloud VMs"}
                </div>
              </div>
            </button>
          );
        })}
      </div>
      {provider === "yandex" && (
        <PlatformSelect value={platformId} onChange={setPlatformId} />
      )}
    </div>
  );
}

// ─── Step 2: Database ────────────────────────────────────────────

function StepDatabase({
  kind, setKind,
  version, setVersion,
  packageId, setPackageId,
  availablePackages,
  presetsForKind,
  selectedPresetId, setSelectedPresetId,
  dbMeta, dbColor,
  allowedKinds,
}: {
  kind: DatabaseKind; setKind: (k: DatabaseKind) => void;
  allowedKinds: DatabaseKind[];
  version: string; setVersion: (v: string) => void;
  packageId: string; setPackageId: (v: string) => void;
  availablePackages: Package[];
  presetsForKind: Preset[];
  selectedPresetId: string; setSelectedPresetId: (v: string) => void;
  dbMeta: { icon: typeof Database; label: string };
  dbColor: { hex: string; text: string; accent: string };
}) {
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Database</h2>
        <p className="text-xs text-zinc-500">Choose the database engine, version, and topology preset.</p>
      </div>

      {/* DB Kind */}
      <div className="grid grid-cols-3 gap-2">
        {allowedKinds.map((k) => {
          const meta = DB_META[k];
          const kColor = DB_COLORS[k];
          const Icon = meta.icon;
          const active = kind === k;
          return (
            <button type="button" key={k} onClick={() => setKind(k)}
              className={`flex items-center gap-2.5 border p-2.5 transition-all cursor-pointer ${
                active ? `${kColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <Icon className={`h-4 w-4 ${active ? kColor.text : "text-zinc-600"}`} />
              <span className={`text-sm font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>{meta.label}</span>
            </button>
          );
        })}
      </div>

      {/* Version + Package */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
          <Select
            value={DB_VERSIONS[kind].includes(version) ? version : "__custom__"}
            onValueChange={(v) => { if (v !== "__custom__") setVersion(v); }}
          >
            <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {DB_VERSIONS[kind].map((v) => (
                <SelectItem key={v} value={v}>{v}</SelectItem>
              ))}
              <SelectItem value="__custom__">Custom...</SelectItem>
            </SelectContent>
          </Select>
          {!DB_VERSIONS[kind].includes(version) && (
            <input
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="Enter version"
              className="h-7 w-full px-2 font-mono text-xs bg-transparent border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
            />
          )}
        </div>
        {kind !== "ydb" && (
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
            <a href="/packages" className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage packages</a>
          </div>
        )}
      </div>

      {/* Topology Preset */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Topology Preset</span>
          <a href="/presets" className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage presets</a>
        </div>
        <div className="grid grid-cols-3 gap-3">
          {presetsForKind.map((p) => {
            const active = selectedPresetId === p.id;
            return (
              <button type="button" key={p.id} onClick={() => setSelectedPresetId(p.id)}
                className={`border p-3 text-left transition-all cursor-pointer ${
                  active ? `${dbColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <span className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? dbColor.text : "text-zinc-400"}`}>
                    {p.name}
                  </span>
                  <div className="flex items-center gap-1">
                    {!p.is_builtin && <span className="text-[8px] text-zinc-600 font-mono">custom</span>}
                    {active && <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: dbColor.hex }} />}
                  </div>
                </div>
                <TopologyDiagram kind={kind} topology={p.topology} />
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ─── Step 3: Stroppy ─────────────────────────────────────────────

// Suggest minimum disk size (GB) based on script type and scale factor.
// TPC-C: ~100 MB per warehouse (data + indexes). TPC-B: ~15 MB per scale unit. 2x headroom for WAL/bloat.
function suggestDiskGb(script: string, scaleFactor: number): { diskGb: number; reason: string } {
  const isTpcc = script.startsWith("tpcc");
  const dataPerUnit = isTpcc ? 100 : 15; // MB
  const rawMb = dataPerUnit * Math.max(1, scaleFactor);
  const withHeadroom = rawMb * 2;
  const diskGb = Math.max(25, Math.ceil(withHeadroom / 1024));
  const reason = `${isTpcc ? "TPC-C" : "TPC-B"} × ${scaleFactor} ≈ ${Math.ceil(rawMb / 1024)} GB data → ${diskGb} GB with headroom`;
  return { diskGb, reason };
}

// Suggest optimal stroppy machine based on VUs and pool size.
function suggestStroppyMachine(vus: number, poolSize: number): { cpus: number; memory: number; disk: number; reason: string } {
  // Rule of thumb: 1 vCPU per ~20 VUs, min 2. Snap to valid platform steps.
  const rawCpus = Math.max(2, Math.min(96, Math.ceil(vus / 20) * 2));
  const cpus = closestStep(rawCpus, CPU_STEPS);
  const memBase = cpus * 2048;
  const memPool = Math.ceil(poolSize / 100) * 512;
  const memory = Math.max(2048, memBase + memPool);
  const disk = 25;
  const reason = `${vus} VUs → ${cpus} vCPU, pool ${poolSize} → ${(memory/1024).toFixed(0)} GB RAM`;
  return { cpus, memory, disk, reason };
}

function StepStroppy({
  script, setScript,
  duration, setDuration,
  scaleFactor, setScaleFactor,
  vus, setVus,
  poolSize, setPoolSize,
  dbKind,
  provider,
  platformId,
  quotas,
  availableSteps, setAvailableSteps,
  selectedSteps, setSelectedSteps,
  noSteps, setNoSteps,
  probeData, setProbeData,
  dbCpus, setDbCpus,
  dbMemory, setDbMemory,
  dbDisk, setDbDisk,
  dbDiskType, setDbDiskType,
  stroppyVersion, setStroppyVersion,
  stroppyVersions, setStroppyVersions,
  versionsLoading, setVersionsLoading,
  versionsLoaded,
}: {
  script: string; setScript: (v: string) => void;
  duration: string; setDuration: (v: string) => void;
  scaleFactor: number; setScaleFactor: (v: number) => void;
  vus: number; setVus: (v: number) => void;
  poolSize: number; setPoolSize: (v: number) => void;
  dbKind: DatabaseKind;
  provider: Provider;
  platformId: string;
  quotas: TenantQuotas;
  availableSteps: string[]; setAvailableSteps: (v: string[]) => void;
  selectedSteps: string[]; setSelectedSteps: (v: string[]) => void;
  noSteps: string[]; setNoSteps: (v: string[]) => void;
  probeData: ProbeResponse | null; setProbeData: (v: ProbeResponse | null) => void;
  dbCpus: number; setDbCpus: (v: number) => void;
  dbMemory: number; setDbMemory: (v: number) => void;
  dbDisk: number; setDbDisk: (v: number) => void;
  dbDiskType: string; setDbDiskType: (v: string) => void;
  stroppyVersion: string; setStroppyVersion: (v: string) => void;
  stroppyVersions: string[]; setStroppyVersions: (v: string[]) => void;
  versionsLoading: boolean; setVersionsLoading: (v: boolean) => void;
  versionsLoaded: React.MutableRefObject<boolean>;
}) {
  // Probe on script change to get steps/env.
  useEffect(() => {
    probeScript({ script, driver_type: dbKind, pool_size: poolSize, scale_factor: scaleFactor })
      .then((data) => {
        setProbeData(data);
        setAvailableSteps(data.steps || []);
      })
      .catch(() => {
        setProbeData(null);
        setAvailableSteps([]);
      });
  }, [script, dbKind]);

  const toggleNoStep = (step: string) => {
    setNoSteps(noSteps.includes(step) ? noSteps.filter((s) => s !== step) : [...noSteps, step]);
  };

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold mb-1">Workload Settings</h2>
          <p className="text-xs text-zinc-500">Choose the benchmark script and tune execution parameters.</p>
        </div>
        {/* Stroppy version selector — lazy loads from GitHub */}
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-zinc-600">stroppy</span>
          <select
            value={stroppyVersion}
            onChange={(e) => setStroppyVersion(e.target.value)}
            onFocus={() => {
              if (!versionsLoaded.current) {
                versionsLoaded.current = true;
                setVersionsLoading(true);
                getStroppyVersions()
                  .then((v) => { if (v.length > 0) setStroppyVersions(v); })
                  .catch(() => {})
                  .finally(() => setVersionsLoading(false));
              }
            }}
            className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 text-[11px] font-mono text-zinc-300 outline-none focus:border-zinc-600"
          >
            {stroppyVersions.map((v) => (
              <option key={v} value={v}>v{v}</option>
            ))}
            {versionsLoading && <option disabled>loading...</option>}
          </select>
        </div>
      </div>

      {/* Script selector */}
      <div>
        <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Script</span>
        <div className="grid grid-cols-2 gap-2">
          {SCRIPTS.map((s) => {
            const supported = s.dbs.includes(dbKind);
            const active = script === s.id;
            return (
              <button type="button" key={s.id}
                onClick={() => supported && setScript(s.id)}
                disabled={!supported}
                className={`border p-3 text-left transition-all ${
                  !supported
                    ? "border-zinc-800/40 opacity-40 cursor-not-allowed"
                    : active
                      ? "border-primary/40 bg-primary/[0.06] cursor-pointer"
                      : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700 cursor-pointer"
                }`}
              >
                <div className={`text-xs font-mono font-semibold ${!supported ? "text-zinc-600" : active ? "text-primary" : "text-zinc-400"}`}>{s.label}</div>
                <div className="text-[10px] text-zinc-600 mt-0.5">
                  {s.desc}
                  {!supported && <span className="text-zinc-700"> — not available for {dbKind}</span>}
                </div>
              </button>
            );
          })}
        </div>
      </div>

      {/* Parameters */}
      <div className="grid grid-cols-2 gap-x-6 gap-y-4">
        <DurationSlider label="Duration" value={duration} onChange={setDuration} />
        <NumericSlider label="VUs" value={vus} min={1} max={1000}
          onChange={setVus} hint="Virtual users (k6 --vus), ~VUs/warehouses per warehouse" />
        <NumericSlider label="Scale Factor" value={scaleFactor} min={1} max={1000}
          onChange={setScaleFactor} hint="TPC-C warehouses / TPC-B branches" />
        <NumericSlider label="Pool Size" value={poolSize} min={10} max={1000} step={10}
          onChange={setPoolSize} hint="DB connections" />
      </div>

      {/* Steps (from probe) */}
      {availableSteps.length > 0 && (
        <div>
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">
            Steps <span className="text-zinc-700">(uncheck to skip)</span>
          </span>
          <div className="flex flex-wrap gap-2">
            {availableSteps.map((step) => {
              const skipped = noSteps.includes(step);
              return (
                <button type="button" key={step} onClick={() => toggleNoStep(step)}
                  className={`px-3 py-1.5 text-xs font-mono border transition-all cursor-pointer ${
                    skipped
                      ? "border-zinc-800/60 text-zinc-600 line-through"
                      : "border-primary/30 text-primary bg-primary/[0.06]"
                  }`}
                >
                  {step}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Database machine override — only for cloud providers */}
      {provider === "yandex" && (
      <div>
        <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Database Machine</span>
        <div className="grid grid-cols-3 gap-3">
          <SliderField label="CPUs" value={dbCpus}
            steps={cpuStepsForPlatform(platformId).filter((s) => !quotas.max_cpus_per_node || s <= quotas.max_cpus_per_node)}
            onChange={setDbCpus} format={(v) => `${v} vCPU`} />
          <SliderField label="Memory" value={dbMemory}
            steps={ramSteps(dbCpus, Math.min(platformLimits(platformId).maxRamMb, (quotas.max_memory_mb_per_node || Infinity)))}
            onChange={setDbMemory}
            format={(v) => v >= 1024 ? `${(v/1024).toFixed(v%1024?1:0)} GB` : `${v} MB`} />
          <SliderField label="Disk" value={dbDisk} steps={diskStepsForType(dbDiskType).filter((s) => !quotas.max_disk_gb_per_node || s <= quotas.max_disk_gb_per_node)}
            onChange={setDbDisk} format={(v) => `${v} GB`} />
        </div>
        <DiskTypeSelect
          value={dbDiskType}
          onChange={setDbDiskType}
          diskSizeGb={dbDisk}
        />
        {(() => {
          const ds = suggestDiskGb(script, scaleFactor);
          const undersized = dbDisk < ds.diskGb;
          return (
            <div className={`mt-2 text-[9px] font-mono ${undersized ? "text-amber-500" : "text-zinc-600"}`}>
              {undersized
                ? `Disk may be undersized: ${ds.reason}. Consider ${ds.diskGb} GB+.`
                : ds.reason}
            </div>
          );
        })()}
      </div>
      )}

      {/* Env parameters from probe — editable */}
      {probeData?.env_declarations && probeData.env_declarations.length > 0 && (() => {
        // Filter out env vars already covered by dedicated UI controls.
        const covered = new Set(["POOL_SIZE", "SCALE_FACTOR", "WAREHOUSES", "STROPPY_STEPS", "STROPPY_NO_STEPS", "STROPPY_ERROR_MODE"]);
        const envDecls = probeData.env_declarations.filter(
          (e) => !e.names.every((n) => covered.has(n))
        );
        if (envDecls.length === 0) return null;
        return (
          <div>
            <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">
              Script Parameters
            </span>
            <div className="space-y-2">
              {envDecls.map((e, i) => {
                const name = e.names[0];
                return (
                  <div key={i} className="flex items-center gap-3">
                    <label className="text-[10px] font-mono text-zinc-400 w-36 shrink-0 truncate" title={e.names.join(", ")}>
                      {name}
                    </label>
                    <input
                      type="text"
                      defaultValue={e.default || ""}
                      placeholder={e.default || ""}
                      className="flex-1 bg-zinc-900 border border-zinc-800 rounded px-2 py-1 text-[11px] font-mono text-zinc-300 outline-none focus:border-zinc-600"
                      data-env-name={name}
                    />
                    {e.description && (
                      <span className="text-[9px] text-zinc-600 truncate max-w-[200px]" title={e.description}>
                        {e.description}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })()}
    </div>
  );
}

// ─── Step 4: Review & Launch ─────────────────────────────────────

// Phase grouping — mirrors DAG dependency structure (same as DagGraph.tsx)
const PHASE_GROUPS: { label: string; icon: typeof Database; phases: string[] }[] = [
  { label: "Infrastructure", icon: Server, phases: ["network", "machines"] },
  { label: "Database", icon: Database, phases: ["install_etcd", "configure_etcd", "install_patroni", "configure_patroni", "install_db", "configure_db", "install_pgbouncer", "configure_pgbouncer"] },
  { label: "Proxy", icon: Server, phases: ["install_proxy", "configure_proxy"] },
  { label: "Monitoring", icon: Server, phases: ["install_monitor", "configure_monitor"] },
  { label: "Benchmark", icon: Rocket, phases: ["install_stroppy", "run_stroppy"] },
  { label: "Teardown", icon: Server, phases: ["teardown"] },
];

function humanPhase(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// Keys that can be edited in the review step — mapped back to state via onEdit.
const EDITABLE_KEYS: Record<string, Set<string>> = {
  database: new Set(["cpu_count", "pdisk_gb", "mem_limit"]),
  benchmark: new Set(["VUs", "duration", "pool", "scale"]),
  infrastructure: new Set(["platform"]),
};

function EditableCfgRow({ k, v, groupKey, onEdit }: {
  k: string; v: string; groupKey: string;
  onEdit?: (group: string, key: string, value: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(v);
  const editable = onEdit && EDITABLE_KEYS[groupKey]?.has(k);

  const commit = () => {
    setEditing(false);
    if (draft !== v && onEdit) onEdit(groupKey, k, draft);
  };

  if (editing) {
    return (
      <div className="flex gap-2 text-[10px] font-mono leading-relaxed">
        <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
        <input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => { if (e.key === "Enter") commit(); if (e.key === "Escape") { setDraft(v); setEditing(false); } }}
          className="flex-1 bg-zinc-800 text-zinc-200 px-1 py-0 border border-zinc-600 outline-none text-[10px] font-mono"
        />
      </div>
    );
  }

  return (
    <div className="flex gap-2 text-[10px] font-mono leading-relaxed group/row">
      <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
      <span className="text-zinc-400 truncate flex-1" title={v}>{v}</span>
      {editable && (
        <button type="button" onClick={() => { setDraft(v); setEditing(true); }}
          className="opacity-0 group-hover/row:opacity-100 text-zinc-600 hover:text-zinc-400 transition-opacity shrink-0">
          <Pencil className="w-2.5 h-2.5" />
        </button>
      )}
    </div>
  );
}

interface DryRunNode {
  id: string;
  type: string;
  deps?: string[];
}

function StepReview({
  dryRunResult,
  dryRunLoading,
  validationResult,
  error,
  submitting,
  onSubmit,
  onEdit,
  dbConfigDraft,
  setDbConfigDraft,
  stroppyConfigDraft,
  setStroppyConfigDraft,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dryRunResult: any;
  dryRunLoading: boolean;
  validationResult: { ok: boolean; message: string } | null;
  error: string | null;
  submitting: boolean;
  onSubmit: () => void;
  onEdit?: (group: string, key: string, value: string) => void;
  dbConfigDraft: string | null;
  setDbConfigDraft: (v: string) => void;
  stroppyConfigDraft: string | null;
  setStroppyConfigDraft: (v: string) => void;
}) {
  const canLaunch = validationResult?.ok && !dryRunLoading && !error;
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggle = (label: string) => setExpanded((prev) => {
    const next = new Set(prev);
    next.has(label) ? next.delete(label) : next.add(label);
    return next;
  });

  // Parse dry-run nodes
  const dagNodes: DryRunNode[] = dryRunResult?.nodes || [];
  const nodeIds = new Set(dagNodes.map((n: DryRunNode) => n.id));

  // Build active groups from plan
  const activeGroups = PHASE_GROUPS
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeIds.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = dagNodes.length;
  const reviewConfigs: Record<string, Record<string, string>> = dryRunResult?.effective_config || {};

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Review & Launch</h2>
        <p className="text-xs text-zinc-500">
          {dryRunLoading ? "Validating configuration and building execution plan..." : "Review the execution plan and launch the run."}
        </p>
      </div>

      {/* Validation status */}
      {dryRunLoading && (
        <div className="flex items-center gap-2 text-xs p-3 border border-zinc-800/50 font-mono text-zinc-500">
          <Loader2 className="h-3 w-3 animate-spin" />
          Preparing execution plan...
        </div>
      )}
      {validationResult && !dryRunLoading && (
        <div className={`flex items-center gap-2 text-xs p-3 border font-mono ${
          validationResult.ok ? "border-emerald-500/30 text-emerald-400" : "border-red-500/30 text-red-400"
        }`}>
          {validationResult.ok ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {validationResult.message}
          {validationResult.ok && totalPhases > 0 && (
            <span className="ml-auto text-zinc-600">{totalPhases} phases</span>
          )}
        </div>
      )}
      {error && (
        <div className="flex items-center gap-2 text-xs p-3 border border-red-500/30 text-red-400 font-mono">
          <AlertCircle className="h-3 w-3" />
          {error}
        </div>
      )}

      {/* Execution plan — accordion groups */}
      {activeGroups.length > 0 && (
        <div className="select-none max-w-xl">
          {activeGroups.map((group, gi) => {
            const isLast = gi === activeGroups.length - 1;
            const GroupIcon = group.icon;
            const isExpanded = expanded.has(group.label);
            const groupKey = group.label.toLowerCase();
            const cfgEntries = reviewConfigs[groupKey];

            return (
              <div key={group.label} className="relative">
                {gi > 0 && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2.5 bg-zinc-800" />
                  </div>
                )}

                <div className="border border-zinc-800/80 bg-zinc-900/30">
                  <button
                    type="button"
                    onClick={() => toggle(group.label)}
                    className="flex items-center gap-2 px-3 py-2 w-full text-left cursor-pointer hover:bg-white/[0.02] transition-colors"
                  >
                    <div className="w-6 h-6 rounded-full bg-zinc-900 border border-zinc-700 flex items-center justify-center shrink-0">
                      <GroupIcon className="w-3 h-3 text-zinc-500" />
                    </div>
                    <span className="text-[11px] font-semibold text-zinc-300 font-mono flex-1">{group.label}</span>
                    <span className="text-[10px] text-zinc-600 font-mono tabular-nums">{group.phases.length}</span>
                    <ChevronDown className={`w-3 h-3 text-zinc-600 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`} />
                  </button>

                  {isExpanded && (
                    <>
                      <div className="border-t border-zinc-800/50">
                        {group.phases.map((phaseId, pi) => {
                          const node = dagNodes.find((n: DryRunNode) => n.id === phaseId);
                          const isLastStep = pi === group.phases.length - 1;
                          const deps = node?.deps?.filter((d: string) => nodeIds.has(d)) || [];

                          return (
                            <div key={phaseId}>
                              <div className="flex items-center gap-2 px-3 py-1.5 relative">
                                {!isLastStep && (
                                  <div className="absolute left-[17px] top-[22px] w-px h-[calc(100%-10px)] bg-zinc-800" />
                                )}
                                <div className="w-[14px] h-[14px] rounded-full border border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0">
                                  <div className="w-1 h-1 rounded-full bg-zinc-600" />
                                </div>
                                <span className="text-[11px] font-mono text-zinc-400 flex-1">{humanPhase(phaseId)}</span>
                                {deps.length > 0 && (
                                  <span className="text-[9px] text-zinc-700 font-mono shrink-0">← {deps.map(humanPhase).join(", ")}</span>
                                )}
                              </div>
                              {!isLastStep && <div className="mx-3 border-b border-zinc-800/20" />}
                            </div>
                          );
                        })}
                      </div>

                      {groupKey === "database" && dbConfigDraft !== null ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50">
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Database Config (editable JSON)</span>
                          </div>
                          <textarea
                            value={dbConfigDraft}
                            onChange={(e) => setDbConfigDraft(e.target.value)}
                            spellCheck={false}
                            className="w-full h-64 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
                          />
                          {cfgEntries && Object.keys(cfgEntries).length > 0 && (
                            <div className="mt-2 space-y-0.5">
                              <span className="text-[9px] font-mono text-zinc-600 uppercase">Effective Config</span>
                              {Object.entries(cfgEntries).map(([k, v]) => (
                                <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                              ))}
                            </div>
                          )}
                        </div>
                      ) : groupKey === "benchmark" && stroppyConfigDraft !== null ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50">
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Stroppy Config (editable protojson — overrides field-level settings)</span>
                          </div>
                          <textarea
                            value={stroppyConfigDraft}
                            onChange={(e) => setStroppyConfigDraft(e.target.value)}
                            spellCheck={false}
                            className="w-full h-72 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
                          />
                          {cfgEntries && Object.keys(cfgEntries).length > 0 && (
                            <div className="mt-2 space-y-0.5">
                              <span className="text-[9px] font-mono text-zinc-600 uppercase">Summary</span>
                              {Object.entries(cfgEntries).map(([k, v]) => (
                                <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                              ))}
                            </div>
                          )}
                        </div>
                      ) : cfgEntries && Object.keys(cfgEntries).length > 0 ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50 space-y-0.5">
                          {Object.entries(cfgEntries).map(([k, v]) => (
                            <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                          ))}
                        </div>
                      ) : null}
                    </>
                  )}
                </div>

                {!isLast && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2 bg-zinc-800" />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Launch */}
      <Button
        size="lg"
        onClick={onSubmit}
        disabled={!canLaunch || submitting}
        className="w-full gap-2 h-12 text-base"
      >
        <Rocket className="h-5 w-5" />
        {submitting ? "Launching..." : "Launch Run"}
      </Button>
    </div>
  );
}
