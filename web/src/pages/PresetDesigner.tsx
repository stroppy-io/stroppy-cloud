import { useState, useEffect, useMemo, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getPreset, createPreset, updatePreset } from "@/api/client";
import type {
  DatabaseKind,
  Preset,
  PostgresTopology,
  MySQLTopology,
  PicodataTopology,
  PicodataTier,
  MachineSpec,
} from "@/api/types";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  ArrowLeft,
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Plus,
  Trash2,
} from "lucide-react";
import { DB_COLORS } from "@/lib/db-colors";

// ─── Validation ──────────────────────────────────────────────────

interface ValidationError {
  field: string;
  message: string;
}

function validateMachine(spec: MachineSpec, label: string): ValidationError[] {
  const errs: ValidationError[] = [];
  if (spec.count < 1) errs.push({ field: `${label}.count`, message: `${label}: count must be >= 1` });
  if (spec.cpus < 1) errs.push({ field: `${label}.cpus`, message: `${label}: CPUs must be >= 1` });
  if (spec.memory_mb < 512) errs.push({ field: `${label}.memory_mb`, message: `${label}: memory must be >= 512 MB` });
  if (spec.disk_gb < 10) errs.push({ field: `${label}.disk_gb`, message: `${label}: disk must be >= 10 GB` });
  return errs;
}

function validatePostgres(t: PostgresTopology): ValidationError[] {
  const errs: ValidationError[] = [];

  // Master required, count = 1
  if (!t.master) {
    errs.push({ field: "master", message: "Master is required" });
  } else {
    if (t.master.count !== 1) errs.push({ field: "master.count", message: "Master count must be exactly 1" });
    errs.push(...validateMachine(t.master, "Master"));
  }

  // Replicas validation
  const totalReplicas = t.replicas?.reduce((s, r) => s + r.count, 0) || 0;
  if (t.replicas?.length) {
    for (let i = 0; i < t.replicas.length; i++) {
      errs.push(...validateMachine(t.replicas[i], `Replica[${i}]`));
    }
  }

  // Patroni → Etcd required
  if (t.patroni && !t.etcd) {
    errs.push({ field: "etcd", message: "Patroni requires Etcd for DCS coordination" });
  }

  // sync_replicas constraints
  if (t.sync_replicas > 0) {
    if (totalReplicas < 1) {
      errs.push({ field: "sync_replicas", message: "Synchronous replication requires at least 1 replica" });
    }
    if (t.sync_replicas > totalReplicas) {
      errs.push({ field: "sync_replicas", message: `sync_replicas (${t.sync_replicas}) exceeds total replicas (${totalReplicas})` });
    }
    if (!t.patroni) {
      errs.push({ field: "sync_replicas", message: "Synchronous replication requires Patroni" });
    }
  }

  // HAProxy
  if (t.haproxy) {
    if (t.haproxy.count < 1) errs.push({ field: "haproxy.count", message: "HAProxy count must be >= 1" });
    errs.push(...validateMachine(t.haproxy, "HAProxy"));
  }

  return errs;
}

function validateMySQL(t: MySQLTopology): ValidationError[] {
  const errs: ValidationError[] = [];

  // Primary required, count = 1
  if (!t.primary) {
    errs.push({ field: "primary", message: "Primary is required" });
  } else {
    if (t.primary.count !== 1) errs.push({ field: "primary.count", message: "Primary count must be exactly 1" });
    errs.push(...validateMachine(t.primary, "Primary"));
  }

  const totalReplicas = t.replicas?.reduce((s, r) => s + r.count, 0) || 0;
  if (t.replicas?.length) {
    for (let i = 0; i < t.replicas.length; i++) {
      errs.push(...validateMachine(t.replicas[i], `Replica[${i}]`));
    }
  }

  // GR and semi_sync mutually exclusive
  if (t.group_replication && t.semi_sync) {
    errs.push({ field: "group_replication", message: "Group Replication and Semi-Sync are mutually exclusive" });
  }

  // GR → minimum 3 nodes (1 primary + 2 replicas), max 9
  if (t.group_replication) {
    const totalNodes = 1 + totalReplicas;
    if (totalNodes < 3) {
      errs.push({ field: "replicas", message: `Group Replication requires minimum 3 nodes (have ${totalNodes}). Add at least ${3 - totalNodes} replica(s)` });
    }
    if (totalNodes > 9) {
      errs.push({ field: "replicas", message: `Group Replication supports maximum 9 nodes (have ${totalNodes})` });
    }
  }

  // Semi-sync → at least 1 replica
  if (t.semi_sync && totalReplicas < 1) {
    errs.push({ field: "semi_sync", message: "Semi-synchronous replication requires at least 1 replica" });
  }

  // ProxySQL
  if (t.proxysql) {
    if (t.proxysql.count < 1) errs.push({ field: "proxysql.count", message: "ProxySQL count must be >= 1" });
    errs.push(...validateMachine(t.proxysql, "ProxySQL"));
  }

  return errs;
}

function validatePicodata(t: PicodataTopology): ValidationError[] {
  const errs: ValidationError[] = [];

  // At least 1 instance
  const totalInstances = t.instances?.reduce((s, i) => s + i.count, 0) || 0;
  if (totalInstances < 1) {
    errs.push({ field: "instances", message: "At least 1 instance is required" });
  }
  if (t.instances?.length) {
    for (let i = 0; i < t.instances.length; i++) {
      errs.push(...validateMachine(t.instances[i], `Instance[${i}]`));
    }
  }

  // replication_factor >= 1
  if (t.replication_factor < 1) {
    errs.push({ field: "replication_factor", message: "Replication factor must be >= 1" });
  }

  // shards >= 1
  if (t.shards < 1) {
    errs.push({ field: "shards", message: "Shards must be >= 1" });
  }

  // instances >= replication_factor × shards (when no tiers)
  if (!t.tiers?.length && t.replication_factor >= 1 && t.shards >= 1) {
    const required = t.replication_factor * t.shards;
    if (totalInstances < required) {
      errs.push({ field: "instances", message: `Need >= ${required} instances (replication_factor × shards = ${t.replication_factor} × ${t.shards}), have ${totalInstances}` });
    }
  }

  // Tiers validation
  if (t.tiers?.length) {
    let tierTotal = 0;
    for (let i = 0; i < t.tiers.length; i++) {
      const tier = t.tiers[i];
      if (!tier.name.trim()) errs.push({ field: `tiers[${i}].name`, message: `Tier ${i + 1}: name is required` });
      if (tier.replication_factor < 1) errs.push({ field: `tiers[${i}].replication_factor`, message: `Tier "${tier.name}": replication_factor must be >= 1` });
      if (tier.count < 1) errs.push({ field: `tiers[${i}].count`, message: `Tier "${tier.name}": count must be >= 1` });
      tierTotal += tier.count;
    }
    const required = t.replication_factor * t.shards;
    if (tierTotal < required) {
      errs.push({ field: "tiers", message: `Sum of tier counts (${tierTotal}) must be >= replication_factor × shards (${required})` });
    }
  }

  // HAProxy
  if (t.haproxy) {
    if (t.haproxy.count < 1) errs.push({ field: "haproxy.count", message: "HAProxy count must be >= 1" });
    errs.push(...validateMachine(t.haproxy, "HAProxy"));
  }

  return errs;
}

// ─── Default topologies ──────────────────────────────────────────

function defaultMachine(role: string, cpus = 2, mem = 4096, disk = 50): MachineSpec {
  return { role: role as MachineSpec["role"], count: 1, cpus, memory_mb: mem, disk_gb: disk };
}

function defaultPostgres(): PostgresTopology {
  return {
    master: defaultMachine("database"),
    replicas: [],
    pgbouncer: false,
    patroni: false,
    etcd: false,
    sync_replicas: 0,
  };
}

function defaultMySQL(): MySQLTopology {
  return {
    primary: defaultMachine("database"),
    replicas: [],
    group_replication: false,
    semi_sync: false,
  };
}

function defaultPicodata(): PicodataTopology {
  return {
    instances: [defaultMachine("database")],
    replication_factor: 1,
    shards: 1,
  };
}

// ─── Machine Spec Editor ─────────────────────────────────────────

function MachineEditor({
  spec,
  onChange,
  label,
  disabled,
  countLocked,
}: {
  spec: MachineSpec;
  onChange: (s: MachineSpec) => void;
  label: string;
  disabled?: boolean;
  countLocked?: boolean;
}) {
  return (
    <div className="border border-zinc-800/60 p-3 space-y-2">
      <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">{label}</span>
      <div className="grid grid-cols-4 gap-2">
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Count</Label>
          <Input type="number" min={1} value={spec.count} onChange={(e) => onChange({ ...spec, count: parseInt(e.target.value) || 1 })}
            className="h-7 text-xs font-mono" disabled={disabled || countLocked} />
        </div>
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">CPUs</Label>
          <Input type="number" min={1} value={spec.cpus} onChange={(e) => onChange({ ...spec, cpus: parseInt(e.target.value) || 1 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Memory MB</Label>
          <Input type="number" min={512} step={512} value={spec.memory_mb} onChange={(e) => onChange({ ...spec, memory_mb: parseInt(e.target.value) || 512 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Disk GB</Label>
          <Input type="number" min={10} value={spec.disk_gb} onChange={(e) => onChange({ ...spec, disk_gb: parseInt(e.target.value) || 10 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
      </div>
    </div>
  );
}

// ─── Toggle Switch ───────────────────────────────────────────────

function Toggle({ label, checked, onChange, disabled }: {
  label: string; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={() => !disabled && onChange(!checked)}
      className={`flex items-center gap-2 border px-3 py-2 transition-all text-left ${
        checked
          ? "border-primary/40 bg-primary/[0.06]"
          : "border-zinc-800/60 hover:border-zinc-700"
      } ${disabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}`}
    >
      <div className={`w-3 h-3 border rounded-sm flex items-center justify-center ${
        checked ? "border-primary bg-primary" : "border-zinc-600"
      }`}>
        {checked && <Check className="w-2 h-2 text-white" />}
      </div>
      <span className={`text-xs font-mono ${checked ? "text-primary" : "text-zinc-500"}`}>{label}</span>
    </button>
  );
}

// ─── Postgres Form ───────────────────────────────────────────────

function PostgresForm({ topology, onChange, disabled }: {
  topology: PostgresTopology;
  onChange: (t: PostgresTopology) => void;
  disabled?: boolean;
}) {
  const addReplica = () => onChange({
    ...topology,
    replicas: [...(topology.replicas || []), { role: "database" as const, count: 2, cpus: 4, memory_mb: 8192, disk_gb: 100 }],
  });
  const removeReplica = (i: number) => onChange({
    ...topology,
    replicas: topology.replicas?.filter((_, j) => j !== i),
  });

  return (
    <div className="space-y-4">
      <MachineEditor label="Master" spec={topology.master} countLocked
        onChange={(s) => onChange({ ...topology, master: s })} disabled={disabled} />

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Replicas</span>
          {!disabled && (
            <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={addReplica}>
              <Plus className="w-3 h-3" /> Add Replica Group
            </Button>
          )}
        </div>
        {topology.replicas?.map((r, i) => (
          <div key={i} className="relative mb-2">
            <MachineEditor label={`Replica group ${i + 1}`} spec={r}
              onChange={(s) => {
                const reps = [...(topology.replicas || [])];
                reps[i] = s;
                onChange({ ...topology, replicas: reps });
              }} disabled={disabled} />
            {!disabled && (
              <button onClick={() => removeReplica(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-4 gap-2">
        <Toggle label="Patroni" checked={topology.patroni} onChange={(v) => onChange({ ...topology, patroni: v, ...(v ? { etcd: true } : {}) })} disabled={disabled} />
        <Toggle label="Etcd" checked={topology.etcd} onChange={(v) => onChange({ ...topology, etcd: v })} disabled={disabled} />
        <Toggle label="PgBouncer" checked={topology.pgbouncer} onChange={(v) => onChange({ ...topology, pgbouncer: v })} disabled={disabled} />
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Sync Replicas</Label>
          <Input type="number" min={0} value={topology.sync_replicas}
            onChange={(e) => onChange({ ...topology, sync_replicas: parseInt(e.target.value) || 0 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
      </div>

      <div>
        <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">HAProxy (optional)</span>
        <div className="flex items-center gap-2 mt-1">
          <Toggle label="Enable HAProxy"
            checked={!!topology.haproxy}
            onChange={(v) => onChange({ ...topology, haproxy: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
            disabled={disabled} />
        </div>
        {topology.haproxy && (
          <div className="mt-2">
            <MachineEditor label="HAProxy" spec={topology.haproxy}
              onChange={(s) => onChange({ ...topology, haproxy: s })} disabled={disabled} />
          </div>
        )}
      </div>
    </div>
  );
}

// ─── MySQL Form ──────────────────────────────────────────────────

function MySQLForm({ topology, onChange, disabled }: {
  topology: MySQLTopology;
  onChange: (t: MySQLTopology) => void;
  disabled?: boolean;
}) {
  const addReplica = () => onChange({
    ...topology,
    replicas: [...(topology.replicas || []), { role: "database" as const, count: 2, cpus: 4, memory_mb: 8192, disk_gb: 100 }],
  });
  const removeReplica = (i: number) => onChange({
    ...topology,
    replicas: topology.replicas?.filter((_, j) => j !== i),
  });

  return (
    <div className="space-y-4">
      <MachineEditor label="Primary" spec={topology.primary} countLocked
        onChange={(s) => onChange({ ...topology, primary: s })} disabled={disabled} />

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Replicas</span>
          {!disabled && (
            <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={addReplica}>
              <Plus className="w-3 h-3" /> Add Replica Group
            </Button>
          )}
        </div>
        {topology.replicas?.map((r, i) => (
          <div key={i} className="relative mb-2">
            <MachineEditor label={`Replica group ${i + 1}`} spec={r}
              onChange={(s) => {
                const reps = [...(topology.replicas || [])];
                reps[i] = s;
                onChange({ ...topology, replicas: reps });
              }} disabled={disabled} />
            {!disabled && (
              <button onClick={() => removeReplica(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-2">
        <Toggle label="Group Replication" checked={topology.group_replication}
          onChange={(v) => onChange({ ...topology, group_replication: v, ...(v ? { semi_sync: false } : {}) })} disabled={disabled} />
        <Toggle label="Semi-Sync Replication" checked={topology.semi_sync}
          onChange={(v) => onChange({ ...topology, semi_sync: v, ...(v ? { group_replication: false } : {}) })} disabled={disabled} />
      </div>

      <div>
        <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">ProxySQL (optional)</span>
        <div className="flex items-center gap-2 mt-1">
          <Toggle label="Enable ProxySQL"
            checked={!!topology.proxysql}
            onChange={(v) => onChange({ ...topology, proxysql: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
            disabled={disabled} />
        </div>
        {topology.proxysql && (
          <div className="mt-2">
            <MachineEditor label="ProxySQL" spec={topology.proxysql}
              onChange={(s) => onChange({ ...topology, proxysql: s })} disabled={disabled} />
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Picodata Form ───────────────────────────────────────────────

function PicodataForm({ topology, onChange, disabled }: {
  topology: PicodataTopology;
  onChange: (t: PicodataTopology) => void;
  disabled?: boolean;
}) {
  const addInstance = () => onChange({
    ...topology,
    instances: [...topology.instances, { role: "database" as const, count: 3, cpus: 4, memory_mb: 8192, disk_gb: 100 }],
  });
  const removeInstance = (i: number) => onChange({
    ...topology,
    instances: topology.instances.filter((_, j) => j !== i),
  });

  const addTier = () => onChange({
    ...topology,
    tiers: [...(topology.tiers || []), { name: "", replication_factor: 1, can_vote: true, count: 3 }],
  });
  const removeTier = (i: number) => onChange({
    ...topology,
    tiers: topology.tiers?.filter((_, j) => j !== i),
  });
  const updateTier = (i: number, t: PicodataTier) => {
    const tiers = [...(topology.tiers || [])];
    tiers[i] = t;
    onChange({ ...topology, tiers });
  };

  return (
    <div className="space-y-4">
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Instances</span>
          {!disabled && (
            <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={addInstance}>
              <Plus className="w-3 h-3" /> Add Instance Group
            </Button>
          )}
        </div>
        {topology.instances.map((inst, i) => (
          <div key={i} className="relative mb-2">
            <MachineEditor label={`Instance group ${i + 1}`} spec={inst}
              onChange={(s) => {
                const insts = [...topology.instances];
                insts[i] = s;
                onChange({ ...topology, instances: insts });
              }} disabled={disabled} />
            {!disabled && topology.instances.length > 1 && (
              <button onClick={() => removeInstance(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Replication Factor</Label>
          <Input type="number" min={1} value={topology.replication_factor}
            onChange={(e) => onChange({ ...topology, replication_factor: parseInt(e.target.value) || 1 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
        <div className="space-y-1">
          <Label className="text-[9px] font-mono text-zinc-600">Shards</Label>
          <Input type="number" min={1} value={topology.shards}
            onChange={(e) => onChange({ ...topology, shards: parseInt(e.target.value) || 1 })}
            className="h-7 text-xs font-mono" disabled={disabled} />
        </div>
      </div>

      <div>
        <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">HAProxy (optional)</span>
        <div className="flex items-center gap-2 mt-1">
          <Toggle label="Enable HAProxy"
            checked={!!topology.haproxy}
            onChange={(v) => onChange({ ...topology, haproxy: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
            disabled={disabled} />
        </div>
        {topology.haproxy && (
          <div className="mt-2">
            <MachineEditor label="HAProxy" spec={topology.haproxy}
              onChange={(s) => onChange({ ...topology, haproxy: s })} disabled={disabled} />
          </div>
        )}
      </div>

      {/* Tiers */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Tiers (optional, for scale deployments)</span>
          {!disabled && (
            <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={addTier}>
              <Plus className="w-3 h-3" /> Add Tier
            </Button>
          )}
        </div>
        {topology.tiers?.map((tier, i) => (
          <div key={i} className="border border-zinc-800/60 p-3 mb-2 space-y-2 relative">
            <div className="grid grid-cols-4 gap-2">
              <div className="space-y-1">
                <Label className="text-[9px] font-mono text-zinc-600">Name</Label>
                <Input value={tier.name} onChange={(e) => updateTier(i, { ...tier, name: e.target.value })}
                  className="h-7 text-xs font-mono" placeholder="compute" disabled={disabled} />
              </div>
              <div className="space-y-1">
                <Label className="text-[9px] font-mono text-zinc-600">RF</Label>
                <Input type="number" min={1} value={tier.replication_factor}
                  onChange={(e) => updateTier(i, { ...tier, replication_factor: parseInt(e.target.value) || 1 })}
                  className="h-7 text-xs font-mono" disabled={disabled} />
              </div>
              <div className="space-y-1">
                <Label className="text-[9px] font-mono text-zinc-600">Count</Label>
                <Input type="number" min={1} value={tier.count}
                  onChange={(e) => updateTier(i, { ...tier, count: parseInt(e.target.value) || 1 })}
                  className="h-7 text-xs font-mono" disabled={disabled} />
              </div>
              <div className="flex items-end">
                <Toggle label="Can Vote" checked={tier.can_vote}
                  onChange={(v) => updateTier(i, { ...tier, can_vote: v })} disabled={disabled} />
              </div>
            </div>
            {!disabled && (
              <button onClick={() => removeTier(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Main Page ───────────────────────────────────────────────────

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres: { icon: Database, label: "PostgreSQL" },
  mysql:    { icon: Server,   label: "MySQL" },
  picodata: { icon: Cpu,      label: "Picodata" },
};

export function PresetDesigner() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [loading, setLoading] = useState(!!id);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [dbKind, setDbKind] = useState<DatabaseKind>("postgres");
  const [isBuiltin, setIsBuiltin] = useState(false);

  const [pgTopology, setPgTopology] = useState<PostgresTopology>(defaultPostgres());
  const [myTopology, setMyTopology] = useState<MySQLTopology>(defaultMySQL());
  const [picoTopology, setPicoTopology] = useState<PicodataTopology>(defaultPicodata());

  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // Load existing preset
  useEffect(() => {
    if (!id) return;
    getPreset(id)
      .then((p: Preset) => {
        setName(p.name);
        setDescription(p.description);
        setDbKind(p.db_kind);
        setIsBuiltin(p.is_builtin);
        if (p.db_kind === "postgres") setPgTopology(p.topology as PostgresTopology);
        else if (p.db_kind === "mysql") setMyTopology(p.topology as MySQLTopology);
        else if (p.db_kind === "picodata") setPicoTopology(p.topology as PicodataTopology);
      })
      .catch(() => setMessage({ type: "error", text: "Failed to load preset" }))
      .finally(() => setLoading(false));
  }, [id]);

  const currentTopology = useMemo(() => {
    if (dbKind === "postgres") return pgTopology;
    if (dbKind === "mysql") return myTopology;
    return picoTopology;
  }, [dbKind, pgTopology, myTopology, picoTopology]);

  const errors = useMemo((): ValidationError[] => {
    const errs: ValidationError[] = [];
    if (!name.trim()) errs.push({ field: "name", message: "Name is required" });
    if (dbKind === "postgres") errs.push(...validatePostgres(pgTopology));
    else if (dbKind === "mysql") errs.push(...validateMySQL(myTopology));
    else if (dbKind === "picodata") errs.push(...validatePicodata(picoTopology));
    return errs;
  }, [name, dbKind, pgTopology, myTopology, picoTopology]);

  const isValid = errors.length === 0;

  const handleSave = useCallback(async () => {
    if (!isValid) return;
    setSaving(true);
    setMessage(null);
    try {
      const topology = currentTopology;
      if (isEdit) {
        await updatePreset(id!, { name, description, topology });
      } else {
        await createPreset({ name, description, db_kind: dbKind, topology });
      }
      navigate("/presets");
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to save" });
    }
    setSaving(false);
  }, [isValid, isEdit, id, name, description, dbKind, currentTopology, navigate]);

  const dbColor = DB_COLORS[dbKind];

  if (loading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading preset...</div>;
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Top bar */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button onClick={() => navigate("/presets")} className="text-zinc-500 hover:text-zinc-300">
            <ArrowLeft className="w-4 h-4" />
          </button>
          <h1 className="text-sm font-semibold">{isEdit ? "Edit Preset" : "New Preset"}</h1>
          {isBuiltin && <Badge variant="secondary" className="text-[8px]">builtin</Badge>}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate("/presets")}>Cancel</Button>
          {!isBuiltin && (
            <Button size="sm" onClick={handleSave} disabled={saving || !isValid} className="gap-1.5">
              <Check className="h-3 w-3" />
              {saving ? "Saving..." : isEdit ? "Save" : "Create"}
            </Button>
          )}
        </div>
      </div>

      {/* Messages */}
      {message && (
        <div className={`shrink-0 mx-5 mt-3 flex items-center gap-2 text-xs p-2 border font-mono ${
          message.type === "success" ? "border-success/30 text-success" : "border-destructive/30 text-destructive"
        }`}>
          {message.type === "success" ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {message.text}
        </div>
      )}

      {/* Main content */}
      <div className="flex-1 min-h-0 flex overflow-hidden">
        {/* Left — form */}
        <div className="flex-1 min-w-0 overflow-y-auto p-5 space-y-5">
          {/* Name / Description / Kind */}
          <div className="grid grid-cols-3 gap-3">
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)}
                className="h-8 text-xs font-mono" placeholder="My HA Postgres" disabled={isBuiltin} />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Description</Label>
              <Input value={description} onChange={(e) => setDescription(e.target.value)}
                className="h-8 text-xs font-mono" placeholder="Custom topology..." disabled={isBuiltin} />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Database</Label>
              <div className="grid grid-cols-3 gap-1">
                {(["postgres", "mysql", "picodata"] as DatabaseKind[]).map((k) => {
                  const meta = DB_META[k];
                  const kColor = DB_COLORS[k];
                  const Icon = meta.icon;
                  const active = dbKind === k;
                  return (
                    <button key={k} type="button"
                      onClick={() => !isEdit && !isBuiltin && setDbKind(k)}
                      className={`flex items-center gap-1.5 border p-1.5 transition-all text-center ${
                        active ? `${kColor.accent}` : "border-zinc-800/60 hover:border-zinc-700"
                      } ${isEdit || isBuiltin ? "cursor-not-allowed opacity-60" : "cursor-pointer"}`}
                    >
                      <Icon className={`h-3 w-3 ${active ? kColor.text : "text-zinc-600"}`} />
                      <span className={`text-[10px] font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>
                        {meta.label}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          {/* Topology form */}
          <div>
            <h2 className={`text-[11px] font-mono uppercase tracking-wider mb-3 ${dbColor.text}`}>
              Topology Configuration
            </h2>
            {dbKind === "postgres" && (
              <PostgresForm topology={pgTopology} onChange={setPgTopology} disabled={isBuiltin} />
            )}
            {dbKind === "mysql" && (
              <MySQLForm topology={myTopology} onChange={setMyTopology} disabled={isBuiltin} />
            )}
            {dbKind === "picodata" && (
              <PicodataForm topology={picoTopology} onChange={setPicoTopology} disabled={isBuiltin} />
            )}
          </div>
        </div>

        {/* Right — preview + validation */}
        <div className="w-80 shrink-0 flex flex-col bg-[#050505] border-l border-zinc-800/50 overflow-hidden">
          {/* Preview */}
          <div className="p-4 border-b border-zinc-800/50">
            <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 mb-3 block">Preview</span>
            <TopologyDiagram kind={dbKind} topology={currentTopology} />
          </div>

          {/* Validation */}
          <div className="flex-1 overflow-y-auto p-4">
            <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 mb-3 block">
              Validation {isValid ? (
                <Badge variant="success" className="text-[8px] ml-1">OK</Badge>
              ) : (
                <Badge variant="destructive" className="text-[8px] ml-1">{errors.length} error{errors.length !== 1 ? "s" : ""}</Badge>
              )}
            </span>
            {errors.length === 0 ? (
              <div className="flex items-center gap-2 text-xs text-success font-mono">
                <Check className="w-3 h-3" />
                Topology is valid
              </div>
            ) : (
              <div className="space-y-1.5">
                {errors.map((e, i) => (
                  <div key={i} className="flex items-start gap-2 text-[11px] font-mono text-destructive">
                    <AlertCircle className="w-3 h-3 shrink-0 mt-0.5" />
                    <span>{e.message}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* JSON preview */}
          <div className="border-t border-zinc-800/50">
            <details className="group">
              <summary className="px-4 py-2 text-[10px] font-mono text-zinc-600 cursor-pointer hover:text-zinc-400 select-none">
                Raw JSON
              </summary>
              <pre className="px-4 pb-3 text-[10px] font-mono text-zinc-500 leading-[1.5] max-h-48 overflow-auto">
                {JSON.stringify(currentTopology, null, 2)}
              </pre>
            </details>
          </div>
        </div>
      </div>
    </div>
  );
}
