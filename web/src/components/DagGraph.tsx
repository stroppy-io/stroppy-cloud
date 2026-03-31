import { useState, useMemo } from "react";
import type { NodeStatus, NodeStatusValue } from "@/api/types";
import { Check, X, Loader2, Circle, ChevronRight, Zap, Server, Database, BarChart3, Play, Trash2 } from "lucide-react";

interface DagGraphProps {
  nodes: NodeStatus[];
}

// Phase grouping — mirrors the DAG dependency structure
const phaseGroups: { label: string; icon: typeof Zap; phases: string[] }[] = [
  {
    label: "Infrastructure",
    icon: Server,
    phases: ["network", "machines"],
  },
  {
    label: "Database",
    icon: Database,
    phases: [
      "install_etcd",
      "configure_etcd",
      "install_db",
      "configure_db",
      "install_pgbouncer",
      "configure_pgbouncer",
    ],
  },
  {
    label: "Proxy",
    icon: Zap,
    phases: ["install_proxy", "configure_proxy"],
  },
  {
    label: "Monitoring",
    icon: BarChart3,
    phases: ["install_monitor", "configure_monitor"],
  },
  {
    label: "Benchmark",
    icon: Play,
    phases: ["install_stroppy", "run_stroppy"],
  },
  {
    label: "Teardown",
    icon: Trash2,
    phases: ["teardown"],
  },
];

function groupStatus(
  phases: string[],
  nodeMap: Map<string, NodeStatus>
): NodeStatusValue | "running" {
  const statuses = phases
    .map((p) => nodeMap.get(p)?.status)
    .filter(Boolean) as NodeStatusValue[];
  if (statuses.length === 0) return "pending";
  if (statuses.some((s) => s === "failed")) return "failed";
  if (statuses.every((s) => s === "done")) return "done";
  // If any are done but not all → group is "running"
  if (statuses.some((s) => s === "done")) return "running";
  return "pending";
}

// Determine if a step is the currently running one (first pending after done steps)
function isStepActive(
  phaseId: string,
  allPhases: string[],
  nodeMap: Map<string, NodeStatus>
): boolean {
  const idx = allPhases.indexOf(phaseId);
  const status = nodeMap.get(phaseId)?.status;
  if (status !== "pending") return false;
  // Active if all previous phases in entire dag are done
  for (let i = 0; i < idx; i++) {
    const prev = nodeMap.get(allPhases[i]);
    if (prev && prev.status !== "done") return false;
  }
  return true;
}

function StepStatusIcon({ status, active }: { status: NodeStatusValue; active?: boolean }) {
  if (status === "done") {
    return (
      <div className="w-[18px] h-[18px] rounded-full bg-emerald-500 flex items-center justify-center shrink-0">
        <Check className="w-[10px] h-[10px] text-white" strokeWidth={3} />
      </div>
    );
  }
  if (status === "failed") {
    return (
      <div className="w-[18px] h-[18px] rounded-full bg-red-500 flex items-center justify-center shrink-0">
        <X className="w-[10px] h-[10px] text-white" strokeWidth={3} />
      </div>
    );
  }
  if (active) {
    return (
      <div className="w-[18px] h-[18px] rounded-full bg-amber-500 flex items-center justify-center shrink-0 animate-pulse">
        <Loader2 className="w-[10px] h-[10px] text-white animate-spin" strokeWidth={3} />
      </div>
    );
  }
  return (
    <div className="w-[18px] h-[18px] rounded-full border-2 border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0">
      <Circle className="w-[6px] h-[6px] text-zinc-600" fill="currentColor" />
    </div>
  );
}

function GroupStatusIndicator({ status }: { status: NodeStatusValue | "running" }) {
  if (status === "done") {
    return (
      <div className="w-8 h-8 rounded-full bg-emerald-500/15 border-2 border-emerald-500 flex items-center justify-center shrink-0 shadow-[0_0_16px_rgba(16,185,129,0.2)]">
        <Check className="w-4 h-4 text-emerald-400" strokeWidth={2.5} />
      </div>
    );
  }
  if (status === "failed") {
    return (
      <div className="w-8 h-8 rounded-full bg-red-500/15 border-2 border-red-500 flex items-center justify-center shrink-0 shadow-[0_0_16px_rgba(239,68,68,0.2)]">
        <X className="w-4 h-4 text-red-400" strokeWidth={2.5} />
      </div>
    );
  }
  if (status === "running") {
    return (
      <div className="w-8 h-8 rounded-full bg-amber-500/15 border-2 border-amber-500 flex items-center justify-center shrink-0 animate-pulse shadow-[0_0_16px_rgba(245,158,11,0.2)]">
        <Loader2 className="w-4 h-4 text-amber-400 animate-spin" strokeWidth={2.5} />
      </div>
    );
  }
  return (
    <div className="w-8 h-8 rounded-full bg-zinc-900 border-2 border-zinc-700 flex items-center justify-center shrink-0">
      <Circle className="w-3 h-3 text-zinc-600" fill="currentColor" />
    </div>
  );
}

const connectorColor: Record<string, string> = {
  done: "bg-emerald-500/40",
  failed: "bg-red-500/40",
  running: "bg-amber-500/40",
  pending: "bg-zinc-800",
};

function humanPhase(id: string): string {
  return id
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function DagGraph({ nodes }: DagGraphProps) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  // Build flat list of all phases for "active" detection
  const allPhaseIds = useMemo(
    () => phaseGroups.flatMap((g) => g.phases),
    []
  );

  if (nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-3">
        <Loader2 className="w-5 h-5 text-zinc-600 animate-spin" />
        <span className="text-zinc-600 text-xs font-mono">
          Waiting for execution data...
        </span>
      </div>
    );
  }

  const nodeMap = new Map(nodes.map((n) => [n.id, n]));

  const activeGroups = phaseGroups
    .map((g) => ({
      ...g,
      phases: g.phases.filter((p) => nodeMap.has(p)),
    }))
    .filter((g) => g.phases.length > 0);

  const toggle = (label: string) =>
    setCollapsed((prev) => ({ ...prev, [label]: !prev[label] }));

  return (
    <div className="py-3 px-1 select-none max-w-2xl">
      {activeGroups.map((group, gi) => {
        const gStatus = groupStatus(group.phases, nodeMap);
        const isOpen = !(collapsed[group.label] ?? false);
        const isLast = gi === activeGroups.length - 1;
        const doneInGroup = group.phases.filter(
          (p) => nodeMap.get(p)?.status === "done"
        ).length;
        const GroupIcon = group.icon;

        return (
          <div key={group.label} className="relative">
            {/* Vertical connector between groups */}
            {gi > 0 && (
              <div className="flex justify-start pl-[15px]">
                <div
                  className={`w-0.5 h-5 ${connectorColor[gStatus]} transition-colors duration-500`}
                />
              </div>
            )}

            {/* Group card */}
            <div
              className={`border transition-colors duration-300 ${
                gStatus === "done"
                  ? "border-emerald-500/20 bg-emerald-500/[0.02]"
                  : gStatus === "failed"
                    ? "border-red-500/20 bg-red-500/[0.02]"
                    : gStatus === "running"
                      ? "border-amber-500/20 bg-amber-500/[0.02]"
                      : "border-zinc-800/80 bg-zinc-900/30"
              }`}
            >
              {/* Group header — clickable */}
              <button
                onClick={() => toggle(group.label)}
                className="flex items-center gap-3 w-full px-4 py-3 group cursor-pointer hover:bg-white/[0.02] transition-colors"
              >
                <GroupStatusIndicator status={gStatus} />

                <div className="flex items-center gap-2 flex-1 min-w-0">
                  <GroupIcon className="w-3.5 h-3.5 text-zinc-500 shrink-0" />
                  <span className="text-[13px] font-semibold text-zinc-200 group-hover:text-white transition-colors font-mono">
                    {group.label}
                  </span>
                </div>

                {/* Progress counter */}
                <span className="text-[11px] text-zinc-600 font-mono tabular-nums shrink-0">
                  {doneInGroup}/{group.phases.length}
                </span>

                <ChevronRight
                  className={`w-3.5 h-3.5 text-zinc-600 transition-transform duration-200 shrink-0 ${
                    isOpen ? "rotate-90" : ""
                  }`}
                />
              </button>

              {/* Steps list */}
              {isOpen && (
                <div className="border-t border-zinc-800/50">
                  {group.phases.map((phaseId, pi) => {
                    const node = nodeMap.get(phaseId);
                    if (!node) return null;
                    const active = isStepActive(phaseId, allPhaseIds, nodeMap);
                    const isLastStep = pi === group.phases.length - 1;

                    return (
                      <div key={phaseId}>
                        <div
                          className={`flex items-start gap-3 px-4 py-2.5 relative ${
                            active
                              ? "bg-amber-500/[0.04]"
                              : "hover:bg-white/[0.015]"
                          } transition-colors`}
                        >
                          {/* Vertical step connector */}
                          {!isLastStep && (
                            <div
                              className={`absolute left-[21px] top-[30px] w-0.5 h-[calc(100%-14px)] ${
                                node.status === "done"
                                  ? "bg-emerald-500/30"
                                  : "bg-zinc-800"
                              } transition-colors duration-500`}
                            />
                          )}

                          {/* Step connector from group line */}
                          <div className="flex items-center pt-[2px]">
                            <StepStatusIcon status={node.status} active={active} />
                          </div>

                          <div className="flex-1 min-w-0 pt-px">
                            <div className="flex items-center gap-2">
                              <span
                                className={`text-xs font-mono leading-none ${
                                  node.status === "done"
                                    ? "text-zinc-300"
                                    : node.status === "failed"
                                      ? "text-red-400"
                                      : active
                                        ? "text-amber-300"
                                        : "text-zinc-500"
                                }`}
                              >
                                {humanPhase(phaseId)}
                              </span>

                              {node.status === "done" && (
                                <span className="text-[10px] text-emerald-600 font-mono">
                                  done
                                </span>
                              )}
                              {node.status === "failed" && (
                                <span className="text-[10px] text-red-500 font-mono">
                                  failed
                                </span>
                              )}
                              {active && (
                                <span className="text-[10px] text-amber-500 font-mono">
                                  running...
                                </span>
                              )}
                            </div>

                            {/* Error detail */}
                            {node.status === "failed" && node.error && (
                              <div className="mt-2 p-2.5 bg-red-500/5 border border-red-500/20 text-[11px] font-mono text-red-400/90 leading-relaxed max-h-32 overflow-auto">
                                {node.error}
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Separator between steps */}
                        {!isLastStep && (
                          <div className="mx-4 border-b border-zinc-800/30" />
                        )}
                      </div>
                    );
                  })}
                </div>
              )}

              {/* Collapsed dots */}
              {!isOpen && (
                <div className="border-t border-zinc-800/50 px-4 py-2 flex gap-1.5">
                  {group.phases.map((p) => {
                    const s = nodeMap.get(p)?.status || "pending";
                    return (
                      <div
                        key={p}
                        className={`w-2 h-2 rounded-full transition-colors ${
                          s === "done"
                            ? "bg-emerald-500"
                            : s === "failed"
                              ? "bg-red-500"
                              : "bg-zinc-700"
                        }`}
                        title={`${humanPhase(p)}: ${s}`}
                      />
                    );
                  })}
                </div>
              )}
            </div>

            {/* Bottom connector to next group */}
            {!isLast && (
              <div className="flex justify-start pl-[15px]">
                <div
                  className={`w-0.5 h-3 ${
                    gStatus === "done"
                      ? "bg-emerald-500/30"
                      : gStatus === "running"
                        ? "bg-amber-500/30"
                        : "bg-zinc-800"
                  } transition-colors duration-500`}
                />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
