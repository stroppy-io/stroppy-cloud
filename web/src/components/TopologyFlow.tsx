import { useMemo } from "react";
import {
  ReactFlow,
  type Node,
  type Edge,
  Position,
  MarkerType,
  Background,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type {
  RunConfig,
  PostgresTopology,
  MySQLTopology,
  PicodataTopology,
  YDBTopology,
} from "@/api/types";
import { DB_COLORS } from "@/lib/db-colors";

const INFRA_PROXY = "#A0860A";
const INFRA_COORD = "#7C6CC8";
const INFRA_POOL = "#2A7A5A";

interface TopoNode {
  id: string;
  label: string;
  count: number;
  color: string;
  group: "db" | "proxy" | "coord" | "pool";
}

interface TopoEdge {
  from: string;
  to: string;
  label?: string;
  animated?: boolean;
}

function buildPostgres(t: PostgresTopology): { nodes: TopoNode[]; edges: TopoEdge[] } {
  const c = DB_COLORS.postgres;
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  nodes.push({ id: "master", label: "Master", count: 1, color: c.hex, group: "db" });

  const replicaCount = t.replicas?.reduce((s, r) => s + r.count, 0) || 0;
  if (replicaCount > 0) {
    nodes.push({ id: "replicas", label: `Replica${replicaCount > 1 ? "s" : ""}`, count: replicaCount, color: c.hexSecondary, group: "db" });
    edges.push({ from: "master", to: "replicas", label: t.sync_replicas > 0 ? "sync" : "async", animated: true });
  }

  if (t.patroni) {
    nodes.push({ id: "patroni", label: "Patroni", count: 1 + replicaCount, color: INFRA_COORD, group: "coord" });
    edges.push({ from: "patroni", to: "master", label: "manages" });
    if (replicaCount > 0) edges.push({ from: "patroni", to: "replicas", label: "manages" });
  }

  if (t.etcd) {
    nodes.push({ id: "etcd", label: "Etcd", count: 3, color: INFRA_COORD, group: "coord" });
    if (t.patroni) edges.push({ from: "patroni", to: "etcd", label: "DCS" });
  }

  if (t.haproxy) {
    nodes.push({ id: "haproxy", label: "HAProxy", count: t.haproxy.count, color: INFRA_PROXY, group: "proxy" });
    edges.push({ from: "haproxy", to: "master", label: "write" });
    if (replicaCount > 0) edges.push({ from: "haproxy", to: "replicas", label: "read" });
  }

  if (t.pgbouncer) {
    nodes.push({ id: "pgbouncer", label: "PgBouncer", count: 1 + replicaCount, color: INFRA_POOL, group: "pool" });
    const target = t.haproxy ? "haproxy" : "master";
    edges.push({ from: "pgbouncer", to: target, label: "pool" });
  }

  return { nodes, edges };
}

function buildMySQL(t: MySQLTopology): { nodes: TopoNode[]; edges: TopoEdge[] } {
  const c = DB_COLORS.mysql;
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  nodes.push({ id: "primary", label: "Primary", count: 1, color: c.hex, group: "db" });

  const replicaCount = t.replicas?.reduce((s, r) => s + r.count, 0) || 0;
  if (replicaCount > 0) {
    nodes.push({ id: "replicas", label: `Replica${replicaCount > 1 ? "s" : ""}`, count: replicaCount, color: c.hexSecondary, group: "db" });
    const replLabel = t.group_replication ? "GR" : t.semi_sync ? "semi-sync" : "async";
    edges.push({ from: "primary", to: "replicas", label: replLabel, animated: true });
  }

  if (t.proxysql) {
    nodes.push({ id: "proxysql", label: "ProxySQL", count: t.proxysql.count, color: INFRA_PROXY, group: "proxy" });
    edges.push({ from: "proxysql", to: "primary", label: "write" });
    if (replicaCount > 0) edges.push({ from: "proxysql", to: "replicas", label: "read" });
  }

  return { nodes, edges };
}

function buildPicodata(t: PicodataTopology): { nodes: TopoNode[]; edges: TopoEdge[] } {
  const c = DB_COLORS.picodata;
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  const totalInstances = t.instances?.reduce((s, i) => s + i.count, 0) || 0;

  if (t.tiers?.length) {
    for (const tier of t.tiers) {
      const id = `tier-${tier.name}`;
      nodes.push({ id, label: tier.name, count: tier.count, color: tier.can_vote ? c.hex : c.hexSecondary, group: "db" });
    }
    // Connect tiers to each other for replication
    for (let i = 1; i < t.tiers.length; i++) {
      edges.push({ from: `tier-${t.tiers[0].name}`, to: `tier-${t.tiers[i].name}`, label: "replicate", animated: true });
    }
  } else {
    nodes.push({ id: "instances", label: "Instances", count: totalInstances, color: c.hex, group: "db" });
  }

  if (t.haproxy) {
    nodes.push({ id: "haproxy", label: "HAProxy", count: t.haproxy.count, color: INFRA_PROXY, group: "proxy" });
    const target = t.tiers?.length ? `tier-${t.tiers[0].name}` : "instances";
    edges.push({ from: "haproxy", to: target, label: "pgproto" });
  }

  return { nodes, edges };
}

function buildYDB(t: YDBTopology): { nodes: TopoNode[]; edges: TopoEdge[] } {
  const c = DB_COLORS.ydb;
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  const storageCount = t.storage?.count || 1;
  nodes.push({ id: "storage", label: "Storage", count: storageCount, color: c.hex, group: "db" });

  if (t.database) {
    const dbCount = t.database.count || 1;
    nodes.push({ id: "database", label: "Database", count: dbCount, color: c.hexSecondary, group: "db" });
    edges.push({ from: "storage", to: "database", label: "grpc", animated: true });
  }

  if (t.haproxy) {
    nodes.push({ id: "haproxy", label: "HAProxy", count: t.haproxy.count, color: INFRA_PROXY, group: "proxy" });
    const target = t.database ? "database" : "storage";
    edges.push({ from: "haproxy", to: target, label: "grpc" });
  }

  return { nodes, edges };
}

// Layout: position nodes in layers (proxy → db → coord/pool)
function layoutNodes(topoNodes: TopoNode[], topoEdges: TopoEdge[]): { nodes: Node[]; edges: Edge[] } {
  const layers: Record<string, TopoNode[]> = { proxy: [], db: [], coord: [], pool: [] };
  for (const n of topoNodes) layers[n.group].push(n);

  const layerOrder = ["proxy", "pool", "db", "coord"];
  const activeLayers = layerOrder.filter((l) => layers[l].length > 0);

  const NODE_W = 160;
  const NODE_H = 52;
  const GAP_X = 60;
  const GAP_Y = 80;

  const totalWidth = Math.max(...activeLayers.map((l) => layers[l].length)) * (NODE_W + GAP_X);

  const nodes: Node[] = [];
  let y = 40;

  for (const layerKey of activeLayers) {
    const layerNodes = layers[layerKey];
    const rowWidth = layerNodes.length * NODE_W + (layerNodes.length - 1) * GAP_X;
    let x = (totalWidth - rowWidth) / 2 + 40;

    for (const tn of layerNodes) {
      nodes.push({
        id: tn.id,
        position: { x, y },
        data: { label: `${tn.label}${tn.count > 1 ? ` ×${tn.count}` : ""}` },
        style: {
          background: tn.color + "15",
          border: `1px solid ${tn.color}50`,
          color: tn.color,
          borderRadius: "4px",
          fontSize: "12px",
          fontFamily: "JetBrains Mono, monospace",
          fontWeight: 600,
          width: NODE_W,
          height: NODE_H,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        },
        sourcePosition: Position.Bottom,
        targetPosition: Position.Top,
      });
      x += NODE_W + GAP_X;
    }
    y += NODE_H + GAP_Y;
  }

  const edges: Edge[] = topoEdges.map((e, i) => ({
    id: `e-${i}`,
    source: e.from,
    target: e.to,
    label: e.label,
    animated: e.animated || false,
    type: "smoothstep",
    markerEnd: { type: MarkerType.ArrowClosed, width: 12, height: 12 },
    style: { stroke: "#525252", strokeWidth: 1 },
    labelStyle: { fontSize: 10, fontFamily: "JetBrains Mono, monospace", fill: "#71717a" },
    labelBgStyle: { fill: "#0a0a0a", fillOpacity: 0.9 },
    labelBgPadding: [4, 2] as [number, number],
  }));

  return { nodes, edges };
}

export function TopologyFlow({ config }: { config: RunConfig | null }) {
  const { nodes, edges } = useMemo(() => {
    if (!config) return { nodes: [], edges: [] };
    const db = config.database;

    let topo: { nodes: TopoNode[]; edges: TopoEdge[] } = { nodes: [], edges: [] };
    if (db.kind === "postgres" && db.postgres) topo = buildPostgres(db.postgres);
    else if (db.kind === "mysql" && db.mysql) topo = buildMySQL(db.mysql);
    else if (db.kind === "picodata" && db.picodata) topo = buildPicodata(db.picodata);
    else if (db.kind === "ydb" && db.ydb) topo = buildYDB(db.ydb);

    return layoutNodes(topo.nodes, topo.edges);
  }, [config]);

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-600 text-xs font-mono">
        No topology data
      </div>
    );
  }

  return (
    <div className="h-full w-full" style={{ minHeight: 400 }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll
        minZoom={0.5}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#1a1a1a" />
      </ReactFlow>
    </div>
  );
}
