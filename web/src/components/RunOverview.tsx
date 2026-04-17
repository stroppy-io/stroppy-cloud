import { useMemo, useState, useEffect } from "react";
import type { NodeStatus, NodeStatusValue, RunConfig, MachineSpec, DatabaseKind } from "@/api/types";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  Check,
  X,
  Loader2,
  Circle,
  Server,
  Database,
  Zap,
  BarChart3,
  Play,
  Trash2,
  Copy,
  Cpu,
  HardDrive,
  MemoryStick,
  Clock,
  Users,
  Network,
  Container,
  Tag,
  Timer,
  FileCode,
  ChevronDown,
} from "lucide-react";

// ─── Phase groups ────────────────────────────────────────────────

const phaseGroups: { label: string; icon: typeof Zap; phases: string[] }[] = [
  { label: "Infrastructure", icon: Server, phases: ["network", "machines"] },
  {
    label: "Database",
    icon: Database,
    phases: [
      "install_etcd", "configure_etcd",
      "install_db", "configure_db",
      "install_pgbouncer", "configure_pgbouncer",
    ],
  },
  { label: "Proxy", icon: Zap, phases: ["install_proxy", "configure_proxy"] },
  { label: "Monitoring", icon: BarChart3, phases: ["install_monitor", "configure_monitor"] },
  { label: "Benchmark", icon: Play, phases: ["install_stroppy", "run_stroppy"] },
  { label: "Teardown", icon: Trash2, phases: ["teardown"] },
];

function groupStatus(
  phases: string[],
  nodeMap: Map<string, NodeStatus>,
): NodeStatusValue {
  const statuses = phases.map((p) => nodeMap.get(p)?.status).filter(Boolean) as NodeStatusValue[];
  if (statuses.length === 0) return "pending";
  if (statuses.some((s) => s === "failed")) return "failed";
  if (statuses.some((s) => s === "cancelled")) return "cancelled";
  if (statuses.every((s) => s === "done")) return "done";
  if (statuses.some((s) => s === "running" || s === "done")) return "running";
  return "pending";
}

function isStepActive(phaseId: string, allPhases: string[], nodeMap: Map<string, NodeStatus>): boolean {
  const idx = allPhases.indexOf(phaseId);
  if (nodeMap.get(phaseId)?.status !== "pending") return false;
  for (let i = 0; i < idx; i++) {
    const prev = nodeMap.get(allPhases[i]);
    if (prev && prev.status !== "done") return false;
  }
  return true;
}

function humanPhase(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// ─── Status colors ───────────────────────────────────────────────

const statusStyles = {
  done: { border: "border-emerald-500/25", bg: "bg-emerald-500/[0.03]", text: "text-emerald-400", connector: "bg-emerald-500/30" },
  failed: { border: "border-red-500/25", bg: "bg-red-500/[0.03]", text: "text-red-400", connector: "bg-red-500/30" },
  cancelled: { border: "border-zinc-600/25", bg: "bg-zinc-500/[0.03]", text: "text-zinc-400", connector: "bg-zinc-600/30" },
  running: { border: "border-amber-500/25", bg: "bg-amber-500/[0.03]", text: "text-amber-400", connector: "bg-amber-500/30" },
  pending: { border: "border-zinc-800/60", bg: "bg-transparent", text: "text-zinc-600", connector: "bg-zinc-800" },
} as const;

// ─── Tiny components ─────────────────────────────────────────────

function StepDot({ status }: { status: NodeStatusValue; active?: boolean; cancelled?: boolean }) {
  if (status === "done")
    return <div className="w-3.5 h-3.5 rounded-full bg-emerald-500 flex items-center justify-center shrink-0"><Check className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "cancelled")
    return <div className="w-3.5 h-3.5 rounded-full bg-zinc-500 flex items-center justify-center shrink-0"><X className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "failed")
    return <div className="w-3.5 h-3.5 rounded-full bg-red-500 flex items-center justify-center shrink-0"><X className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "running")
    return <div className="w-3.5 h-3.5 rounded-full bg-blue-500 flex items-center justify-center shrink-0 animate-pulse"><Loader2 className="w-2 h-2 text-white animate-spin" strokeWidth={3} /></div>;
  return <div className="w-3.5 h-3.5 rounded-full border border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0"><Circle className="w-1 h-1 text-zinc-600" fill="currentColor" /></div>;
}

function CopyButton({ text }: { text: string }) {
  return (
    <button
      type="button"
      onClick={() => navigator.clipboard.writeText(text)}
      className="p-0.5 text-zinc-600 hover:text-zinc-400 transition-colors"
      title="Copy error"
    >
      <Copy className="w-3 h-3" />
    </button>
  );
}

// ─── Config panel ────────────────────────────────────────────────

function ConfigLine({ label, value, icon: Icon }: { label: string; value: React.ReactNode; icon?: typeof Cpu }) {
  if (!value) return null;
  return (
    <div className="flex items-center gap-2 py-[3px]">
      {Icon && <Icon className="w-3 h-3 text-zinc-600 shrink-0" />}
      <span className="text-[11px] text-zinc-500 shrink-0 w-16">{label}</span>
      <span className="text-xs text-zinc-300 font-mono truncate">{value}</span>
    </div>
  );
}

function MachineRow({ spec }: { spec: MachineSpec }) {
  const diskLabel = spec.disk_type === "network-ssd-io-m3" ? "io-m3" : spec.disk_type === "network-ssd" ? "ssd" : "";
  return (
    <div className="flex items-center gap-1.5 text-[11px] font-mono text-zinc-400 flex-wrap">
      <span className="text-zinc-500 w-16 shrink-0">{spec.role}</span>
      <span>{spec.count}x</span>
      <span className="text-zinc-700">·</span>
      <Cpu className="w-3 h-3 text-zinc-600" /><span>{spec.cpus}</span>
      <MemoryStick className="w-3 h-3 text-zinc-600" /><span>{spec.memory_mb >= 1024 ? `${(spec.memory_mb/1024).toFixed(0)}G` : `${spec.memory_mb}M`}</span>
      <HardDrive className="w-3 h-3 text-zinc-600" /><span>{spec.disk_gb}G</span>
      {diskLabel && <span className="text-zinc-600 text-[9px]">{diskLabel}</span>}
    </div>
  );
}

function ElapsedTimer({ startedAt }: { startedAt: string }) {
  const [elapsed, setElapsed] = useState("");
  useEffect(() => {
    const start = new Date(startedAt).getTime();
    if (isNaN(start) || start < 946684800000) return; // invalid or year < 2000
    const tick = () => {
      const sec = Math.max(0, Math.round((Date.now() - start) / 1000));
      if (sec < 60) setElapsed(`${sec}s`);
      else if (sec < 3600) setElapsed(`${Math.floor(sec / 60)}m ${sec % 60}s`);
      else { const h = Math.floor(sec / 3600); setElapsed(`${h}h ${Math.floor((sec % 3600) / 60)}m`); }
    };
    tick();
    const iv = setInterval(tick, 1000);
    return () => clearInterval(iv);
  }, [startedAt]);
  return <>{elapsed}</>;
}

function formatTs(ts?: string): string {
  if (!ts) return "";
  const d = new Date(ts);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
  return d.toLocaleString("en-GB", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

function ConfigPanel({ config, startedAt, finishedAt, isRunning }: {
  config: RunConfig | null;
  startedAt?: string;
  finishedAt?: string;
  isRunning?: boolean;
}) {
  if (!config) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-600 text-xs font-mono">
        No config data
      </div>
    );
  }

  const db = config.database;
  const topology = db.postgres || db.mysql || db.picodata || db.ydb;
  const s = config.stroppy;
  const script = s.script || s.workload || "";
  const vus = s.vus || s.vus_scale || 0;

  // Topology summary
  let topoLabel = "";
  if (db.postgres) {
    const parts: string[] = ["master"];
    if (db.postgres.replicas?.length) parts.push(`${db.postgres.replicas.length}r`);
    if (db.postgres.patroni) parts.push("patroni");
    if (db.postgres.pgbouncer) parts.push("pgb");
    if (db.postgres.etcd) parts.push("etcd");
    topoLabel = parts.join(" + ");
  } else if (db.mysql) {
    const parts: string[] = ["primary"];
    if (db.mysql.replicas?.length) parts.push(`${db.mysql.replicas.length}r`);
    if (db.mysql.group_replication) parts.push("gr");
    if (db.mysql.proxysql) parts.push("proxysql");
    topoLabel = parts.join(" + ");
  } else if (db.picodata) {
    topoLabel = `${db.picodata.shards}sh rf=${db.picodata.replication_factor}`;
  } else if (db.ydb) {
    const parts: string[] = [`${db.ydb.storage.count} storage`];
    if (db.ydb.database) parts.push(`${db.ydb.database.count} database`);
    else parts.push("combined");
    if (db.ydb.haproxy) parts.push("haproxy");
    topoLabel = parts.join(" + ");
  }

  return (
    <div className="flex flex-col h-full">
      {/* Run ID + timing */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[10px] font-mono text-zinc-600 truncate" title={config.id}>{config.id}</div>
        <div className="flex items-center gap-2 mt-1 text-[10px] font-mono text-zinc-500">
          {startedAt && formatTs(startedAt) && (
            <span className="flex items-center gap-1">
              <Clock className="w-3 h-3 text-zinc-600" />
              {formatTs(startedAt)}
            </span>
          )}
        </div>
        {startedAt && (
          <div className="flex items-center gap-1 mt-0.5 text-[10px] font-mono text-zinc-500">
            <Timer className="w-3 h-3 text-zinc-600" />
            {isRunning ? (
              <ElapsedTimer startedAt={startedAt} />
            ) : finishedAt && formatTs(finishedAt) ? (
              (() => {
                const s = new Date(startedAt).getTime();
                const e = new Date(finishedAt).getTime();
                const sec = Math.max(0, Math.round((e - s) / 1000));
                if (sec < 60) return `${sec}s`;
                if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
                return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
              })()
            ) : "—"}
          </div>
        )}
      </div>

      {/* Database section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Database</div>
        <div className="flex items-baseline gap-2">
          <span className="text-sm font-semibold font-mono text-zinc-100">{db.kind}</span>
          <span className="text-xs font-mono text-zinc-500">v{db.version}</span>
        </div>
        {topoLabel && (
          <div className="text-[11px] font-mono text-zinc-500 mt-0.5">{topoLabel}</div>
        )}
      </div>

      {/* Topology card */}
      {topology && (
        <div className="px-3 py-2 border-b border-zinc-800/50">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Topology</div>
          <TopologyDiagram kind={db.kind as DatabaseKind} topology={topology} />
        </div>
      )}

      {/* Workload section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Workload</div>
        <div className="space-y-0">
          <ConfigLine label="script" value={script} icon={FileCode} />
          {s.version && <ConfigLine label="stroppy" value={`v${s.version}`} icon={Tag} />}
          <ConfigLine label="duration" value={s.duration} icon={Clock} />
          {vus > 0 && <ConfigLine label="VUs" value={String(vus)} icon={Users} />}
          {(s.pool_size ?? 0) > 0 && <ConfigLine label="pool" value={String(s.pool_size)} icon={Network} />}
          {(s.scale_factor ?? 0) > 0 && <ConfigLine label="scale" value={String(s.scale_factor)} />}
          {s.steps && s.steps.length > 0 && (
            <div className="text-[10px] font-mono text-zinc-600 mt-1">
              steps: {s.steps.join(", ")}
            </div>
          )}
          {s.no_steps && s.no_steps.length > 0 && (
            <div className="text-[10px] font-mono text-zinc-600 mt-0.5">
              skip: {s.no_steps.join(", ")}
            </div>
          )}
        </div>
      </div>

      {/* Infra section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Infrastructure</div>
        <ConfigLine label="provider" value={config.provider} icon={Container} />
        {config.platform_id && <ConfigLine label="platform" value={config.platform_id} />}
        <ConfigLine label="network" value={config.network?.cidr} icon={Network} />
        {config.network?.zone && <ConfigLine label="zone" value={config.network.zone} />}
      </div>

      {/* Machines */}
      {config.machines && config.machines.length > 0 && (
        <div className="px-3 py-2 flex-1 min-h-0 overflow-y-auto">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Machines</div>
          <div className="space-y-1">
            {config.machines.map((m, i) => (
              <MachineRow key={`${m.role}-${i}`} spec={m} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Effective config per group ──────────────────────────────────



function CfgRow({ k, v }: { k: string; v?: string | null }) {
  if (!v) return null;
  return (
    <div className="flex gap-2 text-[10px] font-mono leading-relaxed">
      <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
      <span className="text-zinc-400 truncate" title={v}>{v}</span>
    </div>
  );
}

// ─── DAG pipeline (vertical) ─────────────────────────────────────

function DagPipeline({ nodes, cancelled, effectiveConfigs, onViewLogs }: { nodes: NodeStatus[]; cancelled?: boolean; effectiveConfigs?: Record<string, Record<string, string>>; onViewLogs?: (phase: string) => void }) {
  const allPhaseIds = useMemo(() => phaseGroups.flatMap((g) => g.phases), []);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const toggle = (label: string) => setExpanded((prev) => {
    const next = new Set(prev);
    next.has(label) ? next.delete(label) : next.add(label);
    return next;
  });

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-full gap-2">
        <Loader2 className="w-4 h-4 text-zinc-600 animate-spin" />
        <span className="text-zinc-600 text-xs font-mono">Waiting...</span>
      </div>
    );
  }

  const nodeMap = new Map(nodes.map((n) => [n.id, n]));
  const activeGroups = phaseGroups
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeMap.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = activeGroups.reduce((s, g) => s + g.phases.length, 0);
  const donePhases = nodes.filter((n) => n.status === "done").length;
  const failedPhases = nodes.filter((n) => n.status === "failed").length;

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Progress bar */}
      <div className="flex items-center gap-3 px-3 py-2 border-b border-zinc-800/50 shrink-0">
        <div className="flex-1 h-1 bg-zinc-800/80 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-700 ease-out rounded-full ${
              failedPhases > 0 ? (cancelled ? "bg-zinc-500" : "bg-red-500") : donePhases === totalPhases ? "bg-emerald-500" : "bg-amber-500"
            }`}
            style={{ width: `${totalPhases > 0 ? (donePhases / totalPhases) * 100 : 0}%` }}
          />
        </div>
        <span className="text-[11px] font-mono text-zinc-500 tabular-nums shrink-0">
          {donePhases}/{totalPhases}
          {failedPhases > 0 && <span className={`ml-1 ${cancelled ? "text-zinc-500" : "text-red-500"}`}>{cancelled ? `${failedPhases} cancelled` : `${failedPhases} err`}</span>}
        </span>
      </div>

      {/* Scrollable pipeline */}
      <div className="flex-1 min-h-0 overflow-y-auto px-3 py-2">
        {activeGroups.map((group, gi) => {
          const rawGStatus = groupStatus(group.phases, nodeMap);
          const gStatus = cancelled && rawGStatus === "failed" ? "cancelled" : rawGStatus;
          const colors = statusStyles[gStatus as keyof typeof statusStyles] || statusStyles.pending;
          const doneInGroup = group.phases.filter((p) => nodeMap.get(p)?.status === "done").length;
          const GroupIcon = group.icon;
          const isLast = gi === activeGroups.length - 1;
          const isExpanded = expanded.has(group.label);

          return (
            <div key={group.label}>
              {/* Connector between groups */}
              {gi > 0 && (
                <div className="flex justify-start pl-[7px]">
                  <div className={`w-px h-2 ${colors.connector} transition-colors duration-500`} />
                </div>
              )}

              {/* Group */}
              <div className={`border ${colors.border} ${colors.bg} transition-all duration-300`}>
                {/* Group header — clickable */}
                <button
                  type="button"
                  onClick={() => toggle(group.label)}
                  className="flex items-center gap-2 px-2.5 py-1.5 w-full text-left cursor-pointer hover:bg-white/[0.02] transition-colors"
                >
                  <GroupIcon className={`w-3.5 h-3.5 ${colors.text} shrink-0`} />
                  <span className="text-xs font-semibold font-mono text-zinc-200 flex-1 truncate">
                    {group.label}
                  </span>
                  <span className="text-[10px] font-mono text-zinc-600 tabular-nums">
                    {doneInGroup}/{group.phases.length}
                  </span>
                  <ChevronDown className={`w-3 h-3 text-zinc-600 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`} />
                </button>

                {/* Expanded content: steps + config */}
                {isExpanded && (
                  <>
                    {/* Steps */}
                    <div className="border-t border-zinc-800/30 px-2.5 py-1 space-y-px">
                      {group.phases.map((phaseId) => {
                        const node = nodeMap.get(phaseId);
                        if (!node) return null;
                        const active = isStepActive(phaseId, allPhaseIds, nodeMap);

                        return (
                          <div key={phaseId}>
                            <div className={`flex items-center gap-2 py-0.5 px-0.5 rounded-sm ${node.status === "running" ? "bg-blue-500/[0.06]" : active ? "bg-amber-500/[0.06]" : ""}`}>
                              <StepDot status={node.status} />
                              <span
                                className={`text-[11px] font-mono leading-tight truncate ${
                                  node.status === "done" ? "text-zinc-400"
                                    : node.status === "running" ? "text-blue-300"
                                    : node.status === "failed" ? "text-red-400"
                                    : node.status === "cancelled" ? "text-zinc-400"
                                    : active ? "text-amber-300" : "text-zinc-600"
                                }`}
                                title={humanPhase(phaseId)}
                              >
                                {humanPhase(phaseId)}
                              </span>
                            </div>

                            {/* Error — full height, copyable, with log link */}
                            {(node.status === "failed" || node.status === "cancelled") && node.error && (
                              <div className="mt-0.5 mb-1 ml-5">
                                <div className={`p-1.5 text-[11px] font-mono leading-relaxed select-text whitespace-pre-wrap break-all ${
                                  node.status === "cancelled"
                                    ? "bg-zinc-500/5 border border-zinc-500/20 text-zinc-400/80"
                                    : "bg-red-500/5 border border-red-500/20 text-red-400/80"
                                }`}>
                                  {node.error}
                                </div>
                                <div className="flex items-center gap-2 mt-1">
                                  <CopyButton text={node.error} />
                                  {onViewLogs && (
                                    <button
                                      type="button"
                                      onClick={() => onViewLogs(phaseId)}
                                      className="text-[9px] font-mono text-primary hover:underline cursor-pointer"
                                    >
                                      View in Logs
                                    </button>
                                  )}
                                </div>
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>

                    {/* Effective config for this group */}
                    {(() => {
                      const groupKey = group.label.toLowerCase();
                      const stored = effectiveConfigs?.[groupKey];
                      if (stored) {
                        return (
                          <div className="border-t border-zinc-800/20 px-2.5 py-1.5 bg-zinc-900/30">
                            <div className="space-y-0.5">
                              {Object.entries(stored).map(([k, v]) => <CfgRow key={k} k={k} v={v} />)}
                            </div>
                          </div>
                        );
                      }
                      return (
                        <div className="border-t border-zinc-800/20 px-2.5 py-1.5 bg-zinc-900/30">
                          <span className="text-[10px] font-mono text-zinc-700">config not available for this run</span>
                        </div>
                      );
                    })()}
                  </>
                )}
              </div>

              {/* Bottom connector */}
              {!isLast && (
                <div className="flex justify-start pl-[7px]">
                  <div className={`w-px h-1.5 ${gStatus === "done" ? "bg-emerald-500/30" : gStatus === "running" ? "bg-amber-500/30" : gStatus === "cancelled" ? "bg-zinc-600/30" : "bg-zinc-800"} transition-colors duration-500`} />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Main export ─────────────────────────────────────────────────

type RunStatusValue = "running" | "cancelling" | "cancelled" | "failed" | "completed" | "pending";

interface RunOverviewProps {
  nodes: NodeStatus[];
  runStatus?: RunStatusValue;
  onViewLogs?: (phase: string) => void;
  snapshot?: {
    started_at?: string;
    finished_at?: string;
    state?: {
      provider?: string;
      run_config?: Record<string, unknown> | string;
      effective_configs?: Record<string, Record<string, string>>;
    };
  } | null;
}

export function RunOverview({ nodes, snapshot, runStatus, onViewLogs }: RunOverviewProps) {
  const config = useMemo<RunConfig | null>(() => {
    const rc = snapshot?.state?.run_config;
    if (!rc) return null;
    try {
      return typeof rc === "string" ? JSON.parse(rc) : (rc as unknown as RunConfig);
    } catch {
      return null;
    }
  }, [snapshot]);

  return (
    <div className="h-full flex overflow-hidden">
      {/* Left — Config panel */}
      <div className="w-60 shrink-0 border-r border-zinc-800/50 overflow-auto">
        {/* Run status + phase counters */}
        {runStatus && (
          <div className="px-3 py-2 border-b border-zinc-800/50 space-y-1">
            <div className={`text-xs font-mono font-semibold flex items-center gap-1.5 ${
              runStatus === "running" ? "text-blue-400" :
              runStatus === "cancelling" ? "text-amber-400" :
              runStatus === "cancelled" ? "text-zinc-400" :
              runStatus === "failed" ? "text-red-400" :
              runStatus === "completed" ? "text-emerald-400" :
              "text-zinc-500"
            }`}>
              <span className={`inline-block w-2 h-2 rounded-full ${
                runStatus === "running" ? "bg-blue-400 animate-pulse" :
                runStatus === "cancelling" ? "bg-amber-400 animate-pulse" :
                runStatus === "cancelled" ? "bg-zinc-500" :
                runStatus === "failed" ? "bg-red-500" :
                runStatus === "completed" ? "bg-emerald-500" :
                "bg-zinc-600"
              }`} />
              {runStatus === "running" ? "Running" :
               runStatus === "cancelling" ? "Cancelling..." :
               runStatus === "cancelled" ? "Cancelled" :
               runStatus === "failed" ? "Failed" :
               runStatus === "completed" ? "Completed" :
               "Pending"}
            </div>
            <div className="flex items-center gap-2.5 text-[10px] font-mono text-zinc-500">
              {(() => {
                const done = nodes.filter(n => n.status === "done").length;
                const running = nodes.filter(n => n.status === "running").length;
                const failed = nodes.filter(n => n.status === "failed").length;
                const cancelled = nodes.filter(n => n.status === "cancelled").length;
                const pending = nodes.filter(n => n.status === "pending").length;
                return (
                  <>
                    {done > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />{done}</span>}
                    {running > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse inline-block" />{running}</span>}
                    {failed > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-red-500 inline-block" />{failed}</span>}
                    {cancelled > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-zinc-500 inline-block" />{cancelled}</span>}
                    {pending > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-zinc-700 inline-block" />{pending}</span>}
                  </>
                );
              })()}
            </div>
          </div>
        )}
        <ConfigPanel
          config={config}
          startedAt={snapshot?.started_at}
          finishedAt={snapshot?.finished_at}
          isRunning={runStatus === "running" || runStatus === "cancelling"}
        />
      </div>

      {/* Right — DAG pipeline */}
      <div className="flex-1 min-w-0">
        <DagPipeline nodes={nodes} cancelled={runStatus === "cancelled"} effectiveConfigs={snapshot?.state?.effective_configs} onViewLogs={onViewLogs} />
      </div>
    </div>
  );
}
