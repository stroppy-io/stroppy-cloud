import type { DatabaseKind } from "@/api/types";
import { Database, Server, Cpu, Shield, Layers, Globe } from "lucide-react";

interface TopologyDiagramProps {
  kind: DatabaseKind;
  preset: string;
}

interface RoleDef {
  label: string;
  count: number;
  color: string;
  icon: typeof Database;
}

function getRoles(kind: DatabaseKind, preset: string): RoleDef[] {
  if (kind === "postgres") {
    switch (preset) {
      case "single":
        return [{ label: "Master", count: 1, color: "#3b82f6", icon: Database }];
      case "ha":
        return [
          { label: "Master", count: 1, color: "#3b82f6", icon: Database },
          { label: "Replica", count: 2, color: "#6366f1", icon: Database },
          { label: "HAProxy", count: 1, color: "#eab308", icon: Globe },
          { label: "Etcd", count: 3, color: "#8b5cf6", icon: Layers },
        ];
      case "scale":
        return [
          { label: "Master", count: 1, color: "#3b82f6", icon: Database },
          { label: "Replica", count: 4, color: "#6366f1", icon: Database },
          { label: "HAProxy", count: 2, color: "#eab308", icon: Globe },
          { label: "Etcd", count: 3, color: "#8b5cf6", icon: Layers },
        ];
    }
  }

  if (kind === "mysql") {
    switch (preset) {
      case "single":
        return [{ label: "Primary", count: 1, color: "#f97316", icon: Server }];
      case "replica":
        return [
          { label: "Primary", count: 1, color: "#f97316", icon: Server },
          { label: "Replica", count: 2, color: "#fb923c", icon: Server },
          { label: "ProxySQL", count: 1, color: "#eab308", icon: Globe },
        ];
      case "group":
        return [
          { label: "Primary", count: 1, color: "#f97316", icon: Server },
          { label: "Replica", count: 2, color: "#fb923c", icon: Server },
          { label: "ProxySQL", count: 2, color: "#eab308", icon: Globe },
        ];
    }
  }

  if (kind === "picodata") {
    switch (preset) {
      case "single":
        return [{ label: "Instance", count: 1, color: "#22c55e", icon: Cpu }];
      case "cluster":
        return [
          { label: "Instance", count: 3, color: "#22c55e", icon: Cpu },
          { label: "HAProxy", count: 1, color: "#eab308", icon: Globe },
        ];
      case "scale":
        return [
          { label: "Compute", count: 3, color: "#22c55e", icon: Cpu },
          { label: "Storage", count: 3, color: "#10b981", icon: Shield },
          { label: "HAProxy", count: 2, color: "#eab308", icon: Globe },
        ];
    }
  }

  return [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];
}

export function TopologyDiagram({ kind, preset }: TopologyDiagramProps) {
  const roles = getRoles(kind, preset);
  const totalNodes = roles.reduce((s, r) => s + r.count, 0);

  return (
    <div className="space-y-1.5">
      {roles.map((role, i) => {
        const Icon = role.icon;
        return (
          <div key={i} className="flex items-center gap-2">
            <Icon className="h-3 w-3 shrink-0" style={{ color: role.color }} />
            <span className="text-[11px] font-mono text-zinc-400 flex-1 truncate">
              {role.label}
            </span>
            {role.count > 1 && (
              <span
                className="text-[10px] font-mono tabular-nums px-1.5 py-px border"
                style={{ borderColor: role.color + "40", color: role.color }}
              >
                ×{role.count}
              </span>
            )}
          </div>
        );
      })}
      <div className="flex items-center gap-1.5 pt-1 border-t border-zinc-800/40">
        <span className="text-[10px] font-mono text-zinc-600 tabular-nums">
          {totalNodes} node{totalNodes !== 1 ? "s" : ""}
        </span>
        {/* Visual dots representing node count */}
        <div className="flex gap-0.5 ml-auto">
          {roles.map((role, ri) =>
            Array.from({ length: Math.min(role.count, 6) }).map((_, j) => (
              <div
                key={`${ri}-${j}`}
                className="w-1.5 h-1.5 rounded-full"
                style={{ backgroundColor: role.color }}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
