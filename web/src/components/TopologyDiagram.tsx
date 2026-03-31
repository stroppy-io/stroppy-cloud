import type { DatabaseKind } from "@/api/types";

interface TopologyDiagramProps {
  kind: DatabaseKind;
  preset: string;
}

interface NodeDef {
  label: string;
  count: number;
  color: string;
}

function getNodes(kind: DatabaseKind, preset: string): NodeDef[] {
  if (kind === "postgres") {
    switch (preset) {
      case "single":
        return [{ label: "PG Master", count: 1, color: "#3b82f6" }];
      case "ha":
        return [
          { label: "PG Master", count: 1, color: "#3b82f6" },
          { label: "PG Replica", count: 2, color: "#6366f1" },
          { label: "HAProxy", count: 1, color: "#eab308" },
          { label: "Etcd", count: 3, color: "#8b5cf6" },
        ];
      case "scale":
        return [
          { label: "PG Master", count: 1, color: "#3b82f6" },
          { label: "PG Replica", count: 4, color: "#6366f1" },
          { label: "HAProxy", count: 2, color: "#eab308" },
          { label: "Etcd", count: 3, color: "#8b5cf6" },
        ];
    }
  }

  if (kind === "mysql") {
    switch (preset) {
      case "single":
        return [{ label: "MySQL Primary", count: 1, color: "#f97316" }];
      case "replica":
        return [
          { label: "MySQL Primary", count: 1, color: "#f97316" },
          { label: "MySQL Replica", count: 2, color: "#fb923c" },
          { label: "ProxySQL", count: 1, color: "#eab308" },
        ];
      case "group":
        return [
          { label: "MySQL Primary", count: 1, color: "#f97316" },
          { label: "MySQL Replica", count: 2, color: "#fb923c" },
          { label: "ProxySQL", count: 2, color: "#eab308" },
        ];
    }
  }

  if (kind === "picodata") {
    switch (preset) {
      case "single":
        return [{ label: "Picodata", count: 1, color: "#22c55e" }];
      case "cluster":
        return [
          { label: "Picodata", count: 3, color: "#22c55e" },
          { label: "HAProxy", count: 1, color: "#eab308" },
        ];
      case "scale":
        return [
          { label: "Picodata Compute", count: 3, color: "#22c55e" },
          { label: "Picodata Storage", count: 3, color: "#10b981" },
          { label: "HAProxy", count: 2, color: "#eab308" },
        ];
    }
  }

  return [{ label: "Node", count: 1, color: "#6b7280" }];
}

export function TopologyDiagram({ kind, preset }: TopologyDiagramProps) {
  const nodes = getNodes(kind, preset);
  const totalNodes = nodes.reduce((s, n) => s + n.count, 0);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-end gap-3 h-24">
        {nodes.map((node, i) => (
          <div key={i} className="flex flex-col items-center gap-1">
            <div className="flex gap-0.5">
              {Array.from({ length: node.count }).map((_, j) => (
                <div
                  key={j}
                  className="w-8 h-8 border flex items-center justify-center text-[9px] font-mono"
                  style={{ borderColor: node.color, color: node.color }}
                >
                  {node.count > 1 ? j + 1 : ""}
                </div>
              ))}
            </div>
            <span className="text-[10px] text-muted-foreground whitespace-nowrap">
              {node.label}
            </span>
          </div>
        ))}
      </div>
      <div className="text-[10px] text-muted-foreground">
        {totalNodes} node{totalNodes !== 1 ? "s" : ""} total
      </div>
    </div>
  );
}
