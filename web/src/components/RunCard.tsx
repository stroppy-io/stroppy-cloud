import { Link } from "react-router-dom";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { NodeStatus, NodeStatusValue } from "@/api/types";

interface RunCardProps {
  runID: string;
  nodes: NodeStatus[];
}

function statusVariant(
  status: NodeStatusValue
): "success" | "destructive" | "pending" | "default" {
  switch (status) {
    case "done":
      return "success";
    case "failed":
      return "destructive";
    case "pending":
      return "pending";
    default:
      return "default";
  }
}

function overallStatus(nodes: NodeStatus[]): {
  label: string;
  variant: "success" | "destructive" | "pending" | "default";
} {
  if (nodes.length === 0)
    return { label: "unknown", variant: "default" };

  const failed = nodes.some((n) => n.status === "failed");
  if (failed) return { label: "failed", variant: "destructive" };

  const allDone = nodes.every((n) => n.status === "done");
  if (allDone) return { label: "done", variant: "success" };

  return { label: "running", variant: "default" };
}

export function RunCard({ runID, nodes }: RunCardProps) {
  const status = overallStatus(nodes);
  const done = nodes.filter((n) => n.status === "done").length;
  const total = nodes.length;

  return (
    <Link to={`/runs/${runID}`}>
      <Card className="hover:border-primary/50 transition-colors cursor-pointer">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="font-mono text-xs">{runID}</CardTitle>
          <Badge variant={status.variant}>{status.label}</Badge>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>
              {done}/{total} phases
            </span>
            <div className="flex-1 h-1 bg-muted overflow-hidden">
              <div
                className="h-full bg-primary transition-all"
                style={{ width: total > 0 ? `${(done / total) * 100}%` : "0%" }}
              />
            </div>
          </div>
          {nodes.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-1">
              {nodes.map((n) => (
                <span
                  key={n.id}
                  className={`inline-block w-2 h-2 ${
                    n.status === "done"
                      ? "bg-success"
                      : n.status === "failed"
                        ? "bg-destructive"
                        : "bg-pending"
                  }`}
                  title={`${n.id}: ${n.status}`}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}
