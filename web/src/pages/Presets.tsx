import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getPresets } from "@/api/client";
import type { PresetsResponse, DatabaseKind } from "@/api/types";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { DBDefaultsPanel } from "@/components/DBDefaultsPanel";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Play, ChevronDown } from "lucide-react";

interface PresetInfo {
  kind: DatabaseKind;
  name: string;
  nodeCount: number;
  features: string[];
}

function extractPresetInfo(data: PresetsResponse): PresetInfo[] {
  const result: PresetInfo[] = [];

  for (const [name, topo] of Object.entries(data.postgres)) {
    const nodes =
      (topo.master?.count || 0) +
      (topo.replicas?.reduce((s: number, r) => s + r.count, 0) || 0) +
      (topo.haproxy?.count || 0);
    const features: string[] = [];
    if (topo.patroni) features.push("Patroni");
    if (topo.pgbouncer) features.push("PgBouncer");
    if (topo.etcd) features.push("Etcd");
    if (topo.haproxy) features.push("HAProxy");
    if (topo.sync_replicas > 0) features.push(`${topo.sync_replicas} sync`);
    result.push({ kind: "postgres", name, nodeCount: nodes, features });
  }

  for (const [name, topo] of Object.entries(data.mysql)) {
    const nodes =
      (topo.primary?.count || 0) +
      (topo.replicas?.reduce((s: number, r) => s + r.count, 0) || 0) +
      (topo.proxysql?.count || 0);
    const features: string[] = [];
    if (topo.group_replication) features.push("Group Replication");
    if (topo.semi_sync) features.push("Semi-Sync");
    if (topo.proxysql) features.push("ProxySQL");
    result.push({ kind: "mysql", name, nodeCount: nodes, features });
  }

  for (const [name, topo] of Object.entries(data.picodata)) {
    const nodes =
      (topo.instances?.reduce((s: number, i) => s + i.count, 0) || 0) +
      (topo.haproxy?.count || 0);
    const features: string[] = [];
    features.push(`${topo.shards} shards`);
    features.push(`rf=${topo.replication_factor}`);
    if (topo.haproxy) features.push("HAProxy");
    if (topo.tiers?.length) features.push(`${topo.tiers.length} tiers`);
    result.push({ kind: "picodata", name, nodeCount: nodes, features });
  }

  return result;
}

const kindColors: Record<DatabaseKind, string> = {
  postgres: "text-blue-400",
  mysql: "text-orange-400",
  picodata: "text-green-400",
};

export function Presets() {
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedDefaults, setExpandedDefaults] = useState<DatabaseKind | null>(null);

  useEffect(() => {
    getPresets()
      .then((data) => setPresets(extractPresetInfo(data)))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="p-6 text-sm text-muted-foreground">Loading presets...</div>
    );
  }

  const grouped: Record<DatabaseKind, PresetInfo[]> = {
    postgres: [],
    mysql: [],
    picodata: [],
  };
  for (const p of presets) {
    grouped[p.kind].push(p);
  }

  return (
    <div className="p-6 space-y-8">
      <div>
        <h1 className="text-lg font-semibold">Topology Presets</h1>
        <p className="text-sm text-muted-foreground">
          Browse available database topologies, configuration defaults, and start a run
        </p>
      </div>

      {(["postgres", "mysql", "picodata"] as DatabaseKind[]).map((kind) => (
        <div key={kind}>
          <div className="flex items-center justify-between mb-3">
            <h2 className={`text-sm font-semibold uppercase tracking-wider ${kindColors[kind]}`}>
              {kind}
            </h2>
            <button
              onClick={() => setExpandedDefaults(expandedDefaults === kind ? null : kind)}
              className="flex items-center gap-1 text-[10px] uppercase tracking-widest text-zinc-600 hover:text-zinc-400 transition-colors"
            >
              Config defaults
              <ChevronDown
                className={`h-3 w-3 transition-transform ${
                  expandedDefaults === kind ? "rotate-180" : ""
                }`}
              />
            </button>
          </div>

          {/* DB Configuration Defaults panel */}
          {expandedDefaults === kind && (
            <div className="mb-4">
              <DBDefaultsPanel kind={kind} />
            </div>
          )}

          <div className="grid grid-cols-3 gap-4">
            {grouped[kind].map((p) => (
              <Card key={`${p.kind}-${p.name}`}>
                <CardHeader className="pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-sm">{p.name}</CardTitle>
                    <Badge variant="secondary">
                      {p.nodeCount} node{p.nodeCount !== 1 ? "s" : ""}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <TopologyDiagram kind={p.kind} preset={p.name} />
                  <div className="flex flex-wrap gap-1">
                    {p.features.map((f) => (
                      <Badge key={f} variant="outline" className="text-[10px]">
                        {f}
                      </Badge>
                    ))}
                  </div>
                  <Link to={`/runs/new?kind=${p.kind}&preset=${p.name}`}>
                    <Button size="sm" variant="outline" className="w-full mt-2">
                      <Play className="h-3 w-3" />
                      Start Run
                    </Button>
                  </Link>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
