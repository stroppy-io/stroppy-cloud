import type { DatabaseKind, PostgresTopology, MySQLTopology, PicodataTopology } from "@/api/types";
import { DB_COLORS } from "@/lib/db-colors";
import { Database, Server, Cpu, Shield, Layers, Globe } from "lucide-react";

interface TopologyDiagramProps {
  kind: DatabaseKind;
  preset?: string;
  topology?: PostgresTopology | MySQLTopology | PicodataTopology;
}

interface RoleDef {
  label: string;
  count: number;
  color: string;
  icon: typeof Database;
}

// Infra roles use a neutral color across all DB types.
const INFRA_PROXY = "#A0860A";
const INFRA_COORD = "#7C6CC8";

function getRolesFromTopology(kind: DatabaseKind, topology: PostgresTopology | MySQLTopology | PicodataTopology): RoleDef[] {
  const c = DB_COLORS[kind];

  if (kind === "postgres") {
    const t = topology as PostgresTopology;
    const roles: RoleDef[] = [];
    if (t.master) roles.push({ label: "Master", count: t.master.count || 1, color: c.hex, icon: Database });
    if (t.replicas?.length) {
      const total = t.replicas.reduce((s, r) => s + r.count, 0);
      if (total > 0) roles.push({ label: "Replica", count: total, color: c.hexSecondary, icon: Database });
    }
    if (t.haproxy) roles.push({ label: "HAProxy", count: t.haproxy.count || 1, color: INFRA_PROXY, icon: Globe });
    if (t.etcd) roles.push({ label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers });
    return roles;
  }

  if (kind === "mysql") {
    const t = topology as MySQLTopology;
    const roles: RoleDef[] = [];
    if (t.primary) roles.push({ label: "Primary", count: t.primary.count || 1, color: c.hex, icon: Server });
    if (t.replicas?.length) {
      const total = t.replicas.reduce((s, r) => s + r.count, 0);
      if (total > 0) roles.push({ label: "Replica", count: total, color: c.hexSecondary, icon: Server });
    }
    if (t.proxysql) roles.push({ label: "ProxySQL", count: t.proxysql.count || 1, color: INFRA_PROXY, icon: Globe });
    return roles;
  }

  if (kind === "picodata") {
    const t = topology as PicodataTopology;
    const roles: RoleDef[] = [];
    if (t.tiers?.length) {
      for (const tier of t.tiers) {
        roles.push({
          label: tier.name.charAt(0).toUpperCase() + tier.name.slice(1),
          count: tier.count,
          color: tier.can_vote ? c.hex : c.hexSecondary,
          icon: tier.can_vote ? Cpu : Shield,
        });
      }
    } else if (t.instances?.length) {
      const total = t.instances.reduce((s, i) => s + i.count, 0);
      roles.push({ label: "Instance", count: total, color: c.hex, icon: Cpu });
    }
    if (t.haproxy) roles.push({ label: "HAProxy", count: t.haproxy.count || 1, color: INFRA_PROXY, icon: Globe });
    return roles;
  }

  return [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];
}

function getRolesFromPresetName(kind: DatabaseKind, preset: string): RoleDef[] {
  const c = DB_COLORS[kind];

  if (kind === "postgres") {
    switch (preset) {
      case "single":
        return [{ label: "Master", count: 1, color: c.hex, icon: Database }];
      case "ha":
        return [
          { label: "Master", count: 1, color: c.hex, icon: Database },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Database },
          { label: "HAProxy", count: 1, color: INFRA_PROXY, icon: Globe },
          { label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers },
        ];
      case "scale":
        return [
          { label: "Master", count: 1, color: c.hex, icon: Database },
          { label: "Replica", count: 4, color: c.hexSecondary, icon: Database },
          { label: "HAProxy", count: 2, color: INFRA_PROXY, icon: Globe },
          { label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers },
        ];
    }
  }

  if (kind === "mysql") {
    switch (preset) {
      case "single":
        return [{ label: "Primary", count: 1, color: c.hex, icon: Server }];
      case "replica":
        return [
          { label: "Primary", count: 1, color: c.hex, icon: Server },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Server },
          { label: "ProxySQL", count: 1, color: INFRA_PROXY, icon: Globe },
        ];
      case "group":
        return [
          { label: "Primary", count: 1, color: c.hex, icon: Server },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Server },
          { label: "ProxySQL", count: 2, color: INFRA_PROXY, icon: Globe },
        ];
    }
  }

  if (kind === "picodata") {
    switch (preset) {
      case "single":
        return [{ label: "Instance", count: 1, color: c.hex, icon: Cpu }];
      case "cluster":
        return [
          { label: "Instance", count: 3, color: c.hex, icon: Cpu },
          { label: "HAProxy", count: 1, color: INFRA_PROXY, icon: Globe },
        ];
      case "scale":
        return [
          { label: "Compute", count: 3, color: c.hex, icon: Cpu },
          { label: "Storage", count: 3, color: c.hexSecondary, icon: Shield },
          { label: "HAProxy", count: 2, color: INFRA_PROXY, icon: Globe },
        ];
    }
  }

  return [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];
}

export function TopologyDiagram({ kind, preset, topology }: TopologyDiagramProps) {
  const roles = topology
    ? getRolesFromTopology(kind, topology)
    : preset
      ? getRolesFromPresetName(kind, preset)
      : [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];

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
