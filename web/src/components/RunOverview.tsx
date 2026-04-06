import { useMemo } from "react";
import type { NodeStatus, NodeStatusValue, RunConfig, MachineSpec } from "@/api/types";
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
): NodeStatusValue | "running" {
  const statuses = phases.map((p) => nodeMap.get(p)?.status).filter(Boolean) as NodeStatusValue[];
  if (statuses.length === 0) return "pending";
  if (statuses.some((s) => s === "failed")) return "failed";
  if (statuses.every((s) => s === "done")) return "done";
  if (statuses.some((s) => s === "done")) return "running";
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
  running: { border: "border-amber-500/25", bg: "bg-amber-500/[0.03]", text: "text-amber-400", connector: "bg-amber-500/30" },
  pending: { border: "border-zinc-800/60", bg: "bg-transparent", text: "text-zinc-600", connector: "bg-zinc-800" },
} as const;

// ─── Tiny components ─────────────────────────────────────────────

function StepDot({ status, active }: { status: NodeStatusValue; active?: boolean }) {
  if (status === "done")
    return <div className="w-3.5 h-3.5 rounded-full bg-emerald-500 flex items-center justify-center shrink-0"><Check className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "failed")
    return <div className="w-3.5 h-3.5 rounded-full bg-red-500 flex items-center justify-center shrink-0"><X className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (active)
    return <div className="w-3.5 h-3.5 rounded-full bg-amber-500 flex items-center justify-center shrink-0 animate-pulse"><Loader2 className="w-2 h-2 text-white animate-spin" strokeWidth={3} /></div>;
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
  return (
    <div className="flex items-center gap-2 text-[11px] font-mono text-zinc-400">
      <span className="text-zinc-500 w-16 shrink-0">{spec.role}</span>
      <span>{spec.count}x</span>
      <span className="text-zinc-600">|</span>
      <Cpu className="w-3 h-3 text-zinc-600" />
      <span>{spec.cpus}</span>
      <MemoryStick className="w-3 h-3 text-zinc-600" />
      <span>{spec.memory_mb}M</span>
      <HardDrive className="w-3 h-3 text-zinc-600" />
      <span>{spec.disk_gb}G</span>
    </div>
  );
}

function ConfigPanel({ config }: { config: RunConfig | null }) {
  if (!config) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-600 text-xs font-mono">
        No config data
      </div>
    );
  }

  const db = config.database;
  const topo = db.postgres || db.mysql || db.picodata;
  const s = config.stroppy;

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
  }

  return (
    <div className="flex flex-col h-full">
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

      {/* Workload section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Workload</div>
        <div className="space-y-0">
          <ConfigLine label="type" value={s.workload?.toUpperCase()} icon={Play} />
          <ConfigLine label="duration" value={s.duration} icon={Clock} />
          {(s.vus_scale ?? 0) > 0 && <ConfigLine label="vus" value={`${s.vus_scale}x`} icon={Users} />}
          {(s.pool_size ?? 0) > 0 && <ConfigLine label="pool" value={s.pool_size} icon={Network} />}
          {(s.scale_factor ?? 0) > 0 && <ConfigLine label="scale" value={s.scale_factor} />}
        </div>
      </div>

      {/* Infra section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Infrastructure</div>
        <ConfigLine label="provider" value={config.provider} icon={Container} />
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

      {/* DB-specific options */}
      {topo && "options" in topo && topo.options && Object.keys(topo.options).length > 0 && (
        <div className="px-3 py-2 border-t border-zinc-800/50 overflow-y-auto max-h-24">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1">Options</div>
          {Object.entries(topo.options).map(([k, v]) => (
            <div key={k} className="text-[11px] font-mono text-zinc-500">
              <span className="text-zinc-400">{k}</span>={v}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ─── DAG pipeline (vertical) ─────────────────────────────────────

function DagPipeline({ nodes }: { nodes: NodeStatus[] }) {
  const allPhaseIds = useMemo(() => phaseGroups.flatMap((g) => g.phases), []);

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
              failedPhases > 0 ? "bg-red-500" : donePhases === totalPhases ? "bg-emerald-500" : "bg-amber-500"
            }`}
            style={{ width: `${totalPhases > 0 ? (donePhases / totalPhases) * 100 : 0}%` }}
          />
        </div>
        <span className="text-[11px] font-mono text-zinc-500 tabular-nums shrink-0">
          {donePhases}/{totalPhases}
          {failedPhases > 0 && <span className="text-red-500 ml-1">{failedPhases} err</span>}
        </span>
      </div>

      {/* Scrollable pipeline */}
      <div className="flex-1 min-h-0 overflow-y-auto px-3 py-2">
        {activeGroups.map((group, gi) => {
          const gStatus = groupStatus(group.phases, nodeMap);
          const colors = statusStyles[gStatus];
          const doneInGroup = group.phases.filter((p) => nodeMap.get(p)?.status === "done").length;
          const GroupIcon = group.icon;
          const isLast = gi === activeGroups.length - 1;

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
                {/* Group header */}
                <div className="flex items-center gap-2 px-2.5 py-1.5">
                  <GroupIcon className={`w-3.5 h-3.5 ${colors.text} shrink-0`} />
                  <span className="text-xs font-semibold font-mono text-zinc-200 flex-1 truncate">
                    {group.label}
                  </span>
                  <span className="text-[10px] font-mono text-zinc-600 tabular-nums">
                    {doneInGroup}/{group.phases.length}
                  </span>
                </div>

                {/* Steps */}
                <div className="border-t border-zinc-800/30 px-2.5 py-1 space-y-px">
                  {group.phases.map((phaseId) => {
                    const node = nodeMap.get(phaseId);
                    if (!node) return null;
                    const active = isStepActive(phaseId, allPhaseIds, nodeMap);

                    return (
                      <div key={phaseId}>
                        <div className={`flex items-center gap-2 py-0.5 px-0.5 rounded-sm ${active ? "bg-amber-500/[0.06]" : ""}`}>
                          <StepDot status={node.status} active={active} />
                          <span
                            className={`text-[11px] font-mono leading-tight truncate ${
                              node.status === "done" ? "text-zinc-400"
                                : node.status === "failed" ? "text-red-400"
                                  : active ? "text-amber-300" : "text-zinc-600"
                            }`}
                            title={humanPhase(phaseId)}
                          >
                            {humanPhase(phaseId)}
                          </span>
                        </div>

                        {/* Error — scrollable, copyable */}
                        {node.status === "failed" && node.error && (
                          <div className="mt-0.5 mb-1 ml-5 flex items-start gap-1">
                            <div className="flex-1 min-w-0 p-1.5 bg-red-500/5 border border-red-500/20 text-[11px] font-mono text-red-400/80 leading-relaxed max-h-20 overflow-auto select-text whitespace-pre-wrap break-all">
                              {node.error}
                            </div>
                            <CopyButton text={node.error} />
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Bottom connector */}
              {!isLast && (
                <div className="flex justify-start pl-[7px]">
                  <div className={`w-px h-1.5 ${gStatus === "done" ? "bg-emerald-500/30" : gStatus === "running" ? "bg-amber-500/30" : "bg-zinc-800"} transition-colors duration-500`} />
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

interface RunOverviewProps {
  nodes: NodeStatus[];
  snapshot?: {
    started_at?: string;
    finished_at?: string;
    state?: {
      provider?: string;
      run_config?: Record<string, unknown> | string;
    };
  } | null;
}

export function RunOverview({ nodes, snapshot }: RunOverviewProps) {
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
      <div className="w-60 shrink-0 border-r border-zinc-800/50 overflow-hidden">
        <ConfigPanel config={config} />
      </div>

      {/* Right — DAG pipeline */}
      <div className="flex-1 min-w-0">
        <DagPipeline nodes={nodes} />
      </div>
    </div>
  );
}
