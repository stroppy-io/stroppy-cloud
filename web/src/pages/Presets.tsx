import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import {
  listPresets,
  deletePreset,
  clonePreset,
} from "@/api/client";
import type { Preset, DatabaseKind } from "@/api/types";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Plus,
  Play,
  Copy,
  Trash2,
  Pencil,
  Check,
  AlertCircle,
} from "lucide-react";

import { DB_COLORS } from "@/lib/db-colors";

export function Presets() {
  const [presets, setPresets] = useState<Preset[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterKind, setFilterKind] = useState<string>("");
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const load = useCallback(async () => {
    try {
      const params = filterKind ? { db_kind: filterKind } : undefined;
      setPresets(await listPresets(params));
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, [filterKind]);

  useEffect(() => { load(); }, [load]);

  async function handleDelete(id: string) {
    if (!confirm("Delete this preset?")) return;
    try {
      await deletePreset(id);
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  async function handleClone(id: string) {
    try {
      const r = await clonePreset(id);
      setMessage({ type: "success", text: `Cloned as "${r.name}"` });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  // Group presets by db_kind for display.
  const grouped: Record<DatabaseKind, Preset[]> = { postgres: [], mysql: [], picodata: [] };
  for (const p of presets) {
    if (grouped[p.db_kind]) grouped[p.db_kind].push(p);
  }

  const kindsToShow = filterKind
    ? [filterKind as DatabaseKind]
    : (["postgres", "mysql", "picodata"] as DatabaseKind[]);

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Topology Presets</h1>
          <p className="text-sm text-muted-foreground">
            Manage database topology templates for runs
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={filterKind || "__all__"} onValueChange={(v) => setFilterKind(v === "__all__" ? "" : v)}>
            <SelectTrigger className="h-8 w-36 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All databases</SelectItem>
              <SelectItem value="postgres">PostgreSQL</SelectItem>
              <SelectItem value="mysql">MySQL</SelectItem>
              <SelectItem value="picodata">Picodata</SelectItem>
            </SelectContent>
          </Select>
          <Link to="/presets/new">
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" /> New Preset
            </Button>
          </Link>
        </div>
      </div>

      {message && (
        <div className={`flex items-center gap-2 text-xs p-2 border font-mono ${
          message.type === "success" ? "border-success/30 text-success" : "border-destructive/30 text-destructive"
        }`}>
          {message.type === "success" ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {message.text}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading presets...</p>
      ) : (
        kindsToShow.map((kind) => {
          const items = grouped[kind];
          if (!items?.length) return null;
          return (
            <div key={kind}>
              <h2 className={`text-sm font-semibold uppercase tracking-wider mb-3 ${DB_COLORS[kind].text}`}>
                {kind}
              </h2>
              <div className="grid grid-cols-3 gap-4">
                {items.map((p) => (
                  <PresetCard
                    key={p.id}
                    preset={p}
                    onClone={() => handleClone(p.id)}
                    onDelete={() => handleDelete(p.id)}
                  />
                ))}
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}

// ─── Preset Card ─────────────────────────────────────────────────

function PresetCard({
  preset,
  onClone,
  onDelete,
}: {
  preset: Preset;
  onClone: () => void;
  onDelete: () => void;
}) {
  const features = extractFeatures(preset);
  const nodeCount = countNodes(preset);

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 min-w-0">
            <CardTitle className="text-sm truncate">{preset.name}</CardTitle>
            {preset.is_builtin && <Badge variant="secondary" className="text-[8px] shrink-0">builtin</Badge>}
          </div>
          <Badge variant="secondary">
            {nodeCount} node{nodeCount !== 1 ? "s" : ""}
          </Badge>
        </div>
        {preset.description && (
          <p className="text-[10px] text-zinc-500 font-mono truncate">{preset.description}</p>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        <TopologyDiagram kind={preset.db_kind} topology={preset.topology} />
        {features.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {features.map((f) => (
              <Badge key={f} variant="outline" className="text-[10px]">
                {f}
              </Badge>
            ))}
          </div>
        )}
        <div className="flex items-center gap-1 pt-1">
          <Link to={`/runs/new?preset_id=${preset.id}`} className="flex-1">
            <Button size="sm" variant="outline" className="w-full">
              <Play className="h-3 w-3" />
              Start Run
            </Button>
          </Link>
          <Link to={`/presets/${preset.id}/edit`} className="p-1.5 text-zinc-600 hover:text-zinc-300" title="Edit">
            <Pencil className="w-3 h-3" />
          </Link>
          <button onClick={onClone} className="p-1.5 text-zinc-600 hover:text-zinc-300" title="Clone">
            <Copy className="w-3 h-3" />
          </button>
          {!preset.is_builtin && (
            <button onClick={onDelete} className="p-1.5 text-zinc-600 hover:text-red-400" title="Delete">
              <Trash2 className="w-3 h-3" />
            </button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// ─── Helpers ─────────────────────────────────────────────────────

function extractFeatures(preset: Preset): string[] {
  const t = preset.topology;
  const features: string[] = [];

  if (preset.db_kind === "postgres") {
    const pg = t as import("@/api/types").PostgresTopology;
    if (pg.patroni) features.push("Patroni");
    if (pg.pgbouncer) features.push("PgBouncer");
    if (pg.etcd) features.push("Etcd");
    if (pg.haproxy) features.push("HAProxy");
    if (pg.sync_replicas > 0) features.push(`${pg.sync_replicas} sync`);
  } else if (preset.db_kind === "mysql") {
    const my = t as import("@/api/types").MySQLTopology;
    if (my.group_replication) features.push("Group Replication");
    if (my.semi_sync) features.push("Semi-Sync");
    if (my.proxysql) features.push("ProxySQL");
  } else if (preset.db_kind === "picodata") {
    const pico = t as import("@/api/types").PicodataTopology;
    features.push(`${pico.shards} shards`);
    features.push(`rf=${pico.replication_factor}`);
    if (pico.haproxy) features.push("HAProxy");
    if (pico.tiers?.length) features.push(`${pico.tiers.length} tiers`);
  }

  return features;
}

function countNodes(preset: Preset): number {
  const t = preset.topology;

  if (preset.db_kind === "postgres") {
    const pg = t as import("@/api/types").PostgresTopology;
    const etcdCount = pg.etcd ? 3 : 0;
    return (pg.master?.count || 0)
      + (pg.replicas?.reduce((s, r) => s + r.count, 0) || 0)
      + (pg.haproxy?.count || 0)
      + etcdCount;
  }
  if (preset.db_kind === "mysql") {
    const my = t as import("@/api/types").MySQLTopology;
    return (my.primary?.count || 0)
      + (my.replicas?.reduce((s, r) => s + r.count, 0) || 0)
      + (my.proxysql?.count || 0);
  }
  if (preset.db_kind === "picodata") {
    const pico = t as import("@/api/types").PicodataTopology;
    return (pico.instances?.reduce((s, i) => s + i.count, 0) || 0)
      + (pico.haproxy?.count || 0);
  }
  return 0;
}
