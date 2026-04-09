import { useState, useEffect, useMemo, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getPreset, createPreset, updatePreset } from "@/api/client";
import {
  ALL_DB_KINDS,
  type DatabaseKind,
  type Preset,
  type PostgresTopology,
  type MySQLTopology,
  type PicodataTopology,
  type PicodataTier,
  type YDBTopology,
  type MachineSpec,
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
  Lock,
  X,
} from "lucide-react";
import { DB_COLORS } from "@/lib/db-colors";
import { SliderField, NumericSlider, closestStep, ramSteps, CPU_STEPS, DISK_STEPS, DiskTypeSelect } from "@/components/ui/sliders";

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

function validateYDB(t: YDBTopology): ValidationError[] {
  const errs: ValidationError[] = [];
  if (t.storage.count < 1) errs.push({ field: "storage.count", message: "At least 1 storage node required" });
  if (t.database && t.database.count < 1) errs.push({ field: "database.count", message: "Database node count must be >= 1" });
  if (t.storage.cpus < 1) errs.push({ field: "storage.cpus", message: "Storage CPUs must be >= 1" });
  if (t.storage.memory_mb < 2048) errs.push({ field: "storage.memory_mb", message: "Storage RAM must be >= 2 GB" });
  if (t.storage.disk_gb < 80) errs.push({ field: "storage.disk_gb", message: "Storage disk must be >= 80 GB" });
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

function defaultYDB(): YDBTopology {
  return {
    storage: { role: "database", count: 1, cpus: 2, memory_mb: 4096, disk_gb: 80 },
    fault_tolerance: "none",
    database_path: "/Root/testdb",
  };
}

// ─── Machine Spec Editor ─────────────────────────────────────────

function MachineEditor({
  spec,
  onChange,
  label,
  disabled,
  countLocked,
  children,
}: {
  spec: MachineSpec;
  onChange: (s: MachineSpec) => void;
  label: string;
  disabled?: boolean;
  countLocked?: boolean;
  children?: React.ReactNode;
}) {
  const memSteps = ramSteps(spec.cpus);
  const mem = closestStep(spec.memory_mb, memSteps);

  return (
    <div className="border border-zinc-800/60 p-3 space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">{label}</span>
        <span className="text-[9px] font-mono text-zinc-600 tabular-nums">
          {spec.count > 1 ? `${spec.count}× ` : ""}{spec.cpus} vCPU / {spec.memory_mb >= 1024 ? `${(spec.memory_mb / 1024).toFixed(spec.memory_mb % 1024 ? 1 : 0)} GB` : `${spec.memory_mb} MB`} / {spec.disk_gb} GB
        </span>
      </div>
      <div className="grid grid-cols-4 gap-3">
        {!countLocked ? (
          <NumericSlider label="Count" value={spec.count} min={1} max={16}
            onChange={(v) => onChange({ ...spec, count: v })} disabled={disabled} />
        ) : (
          <div className="space-y-1">
            <Label className="text-[9px] font-mono text-zinc-600">Count</Label>
            <div className="h-7 flex items-center text-xs font-mono text-zinc-500">1 (fixed)</div>
          </div>
        )}
        <SliderField label="CPUs" value={spec.cpus} steps={CPU_STEPS} disabled={disabled}
          onChange={(v) => {
            const newMem = closestStep(spec.memory_mb, ramSteps(v));
            onChange({ ...spec, cpus: v, memory_mb: newMem });
          }}
          format={(v) => `${v} vCPU`} />
        <SliderField label="Memory" value={mem} steps={memSteps} disabled={disabled}
          onChange={(v) => onChange({ ...spec, memory_mb: v })}
          format={(v) => v >= 1024 ? `${(v / 1024).toFixed(v % 1024 ? 1 : 0)} GB` : `${v} MB`} />
        <SliderField label="Disk" value={spec.disk_gb} steps={DISK_STEPS} disabled={disabled}
          onChange={(v) => onChange({ ...spec, disk_gb: v })}
          format={(v) => `${v} GB`} />
      </div>
      <DiskTypeSelect
        value={spec.disk_type || "network-ssd"}
        onChange={(v) => onChange({ ...spec, disk_type: v })}
        diskSizeGb={spec.disk_gb}
      />
      {children}
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

// ─── DB Config Defaults & Locked ─────────────────────────────────

// ── PostgreSQL ──
const PG_MASTER_LOCKED: Record<string, string> = { wal_level: "replica", max_wal_senders: "10", max_replication_slots: "10", listen_addresses: "'*'" };
const PG_MASTER_LOCKED_HINTS: Record<string, string> = { wal_level: "required for replication", max_wal_senders: "replication connections", max_replication_slots: "replication slots", listen_addresses: "replicas must connect" };
const PG_MASTER_DEFAULTS: Record<string, string> = { shared_buffers: "25%", max_connections: "200", max_wal_size: "4GB", effective_cache_size: "75%", work_mem: "64MB", maintenance_work_mem: "512MB", wal_buffers: "64MB", checkpoint_completion_target: "0.9", random_page_cost: "1.1", effective_io_concurrency: "200" };

const PG_REPLICA_LOCKED: Record<string, string> = { hot_standby: "on", primary_conninfo: "<auto>" };
const PG_REPLICA_LOCKED_HINTS: Record<string, string> = { hot_standby: "required for read replicas", primary_conninfo: "derived from master IP" };
const PG_REPLICA_DEFAULTS: Record<string, string> = { ...PG_MASTER_DEFAULTS };

const HAPROXY_LOCKED: Record<string, string> = { mode: "tcp" };
const HAPROXY_LOCKED_HINTS: Record<string, string> = { mode: "required for DB protocol" };
const HAPROXY_DEFAULTS: Record<string, string> = { maxconn: "4096", "timeout connect": "5s", "timeout client": "30s", "timeout server": "30s", inter: "3s", fall: "3", rise: "2" };

const PGBOUNCER_LOCKED: Record<string, string> = { "listen_addr": "0.0.0.0", "listen_port": "6432", "admin_users": "postgres" };
const PGBOUNCER_LOCKED_HINTS: Record<string, string> = { listen_addr: "accepts all connections", listen_port: "PgBouncer standard port", admin_users: "admin access" };
const PGBOUNCER_DEFAULTS: Record<string, string> = { pool_mode: "transaction", max_client_conn: "1000", default_pool_size: "25", auth_type: "trust" };

const PATRONI_LOCKED: Record<string, string> = { "restapi.listen": "0.0.0.0:8008", "etcd3.hosts": "<auto>" };
const PATRONI_LOCKED_HINTS: Record<string, string> = { "restapi.listen": "health check endpoint", "etcd3.hosts": "derived from Etcd nodes" };
const PATRONI_DEFAULTS: Record<string, string> = { ttl: "30", loop_wait: "10", retry_timeout: "10", maximum_lag_on_failover: "1048576" };

const ETCD_LOCKED: Record<string, string> = { "initial-cluster": "<auto>", "initial-cluster-state": "new" };
const ETCD_LOCKED_HINTS: Record<string, string> = { "initial-cluster": "derived from node list", "initial-cluster-state": "bootstrap mode" };
const ETCD_DEFAULTS: Record<string, string> = { "heartbeat-interval": "100", "election-timeout": "1000", "snapshot-count": "10000" };

// ── MySQL ──
const MYSQL_PRIMARY_LOCKED: Record<string, string> = { "server-id": "<auto>", gtid_mode: "ON", enforce_gtid_consistency: "ON", log_bin: "mysql-bin", bind_address: "0.0.0.0" };
const MYSQL_PRIMARY_LOCKED_HINTS: Record<string, string> = { "server-id": "unique per node", gtid_mode: "required for GTID replication", enforce_gtid_consistency: "required with gtid_mode", log_bin: "required for replication", bind_address: "replicas must connect" };
const MYSQL_PRIMARY_DEFAULTS: Record<string, string> = { innodb_buffer_pool_size: "25%", max_connections: "200", innodb_log_file_size: "1G", innodb_flush_method: "O_DIRECT", innodb_flush_log_at_trx_commit: "1", innodb_io_capacity: "2000", innodb_io_capacity_max: "4000", innodb_read_io_threads: "8", innodb_write_io_threads: "8", table_open_cache: "4000", thread_cache_size: "64" };

const MYSQL_REPLICA_LOCKED: Record<string, string> = { "server-id": "<auto>", gtid_mode: "ON", enforce_gtid_consistency: "ON", "read_only": "ON" };
const MYSQL_REPLICA_LOCKED_HINTS: Record<string, string> = { "server-id": "unique per node", gtid_mode: "required for GTID replication", enforce_gtid_consistency: "required with gtid_mode", read_only: "replica is read-only" };
const MYSQL_REPLICA_DEFAULTS: Record<string, string> = { ...MYSQL_PRIMARY_DEFAULTS };

const PROXYSQL_LOCKED: Record<string, string> = { "listen_port": "6033", "admin_port": "6032", "monitor_username": "monitor" };
const PROXYSQL_LOCKED_HINTS: Record<string, string> = { listen_port: "client connections", admin_port: "admin interface", monitor_username: "backend health checks" };
const PROXYSQL_DEFAULTS: Record<string, string> = { threads: "4", max_connections: "2048", default_query_timeout: "36000000" };

// ── Picodata ──
const PICO_LOCKED: Record<string, string> = { "cluster.name": "stroppy-cluster", "iproto.listen": "0.0.0.0:3301", "pgproto.listen": "0.0.0.0:4327" };
const PICO_LOCKED_HINTS: Record<string, string> = { "cluster.name": "set by system", "iproto.listen": "internal protocol port", "pgproto.listen": "PostgreSQL wire protocol" };
const PICO_DEFAULTS: Record<string, string> = { memtx_memory: "25%", vinyl_memory: "25%", net_msg_max: "1024", readahead: "16384", log_level: "info" };

const PICO_HAPROXY_DEFAULTS: Record<string, string> = { ...HAPROXY_DEFAULTS };

// ─── Options Editor ──────────────────────────────────────────────

function OptionsEditor({
  label,
  options,
  onChange,
  locked,
  lockedHints,
  defaults,
  disabled,
}: {
  label: string;
  options: Record<string, string> | undefined;
  onChange: (opts: Record<string, string> | undefined) => void;
  locked: Record<string, string>;
  lockedHints: Record<string, string>;
  defaults: Record<string, string>;
  disabled?: boolean;
}) {
  const [draftKey, setDraftKey] = useState("");
  const [draftVal, setDraftVal] = useState("");
  const [showSuggestions, setShowSuggestions] = useState(false);

  const opts = options || {};
  const overrideCount = Object.keys(opts).length;

  const addOption = () => {
    const k = draftKey.trim();
    const v = draftVal.trim();
    if (!k || !v || k in locked) return;
    const next = { ...opts, [k]: v };
    onChange(Object.keys(next).length ? next : undefined);
    setDraftKey("");
    setDraftVal("");
    setShowSuggestions(false);
  };

  const removeOption = (key: string) => {
    const next = { ...opts };
    delete next[key];
    onChange(Object.keys(next).length ? next : undefined);
  };

  const updateValue = (key: string, val: string) => {
    onChange({ ...opts, [key]: val });
  };

  const suggestions = Object.keys(defaults).filter(
    (k) => !(k in locked) && !(k in opts) && (!draftKey || k.toLowerCase().includes(draftKey.toLowerCase()))
  );

  return (
    <details className="group border border-zinc-800/60 mt-4">
      <summary className="px-3 py-2 text-[10px] font-mono text-zinc-500 uppercase tracking-wider cursor-pointer hover:text-zinc-400 select-none flex items-center gap-2">
        {label}
        {overrideCount > 0 && (
          <Badge variant="secondary" className="text-[8px]">{overrideCount} override{overrideCount !== 1 ? "s" : ""}</Badge>
        )}
      </summary>
      <div className="px-3 pb-3 space-y-3">

        {/* Locked params */}
        <div className="space-y-1">
          <span className="text-[9px] font-mono text-zinc-600 uppercase tracking-wider">System-managed (cannot change)</span>
          {Object.entries(locked).map(([k, v]) => (
            <div key={k} className="flex items-center gap-2 text-[10px] font-mono text-zinc-600">
              <Lock className="w-2.5 h-2.5 shrink-0" />
              <span className="text-zinc-500">{k}</span>
              <span className="text-zinc-700">=</span>
              <span>{v}</span>
              {lockedHints[k] && <span className="text-zinc-700 text-[9px]">— {lockedHints[k]}</span>}
            </div>
          ))}
        </div>

        {/* Defaults reference */}
        <div className="space-y-1">
          <span className="text-[9px] font-mono text-zinc-600 uppercase tracking-wider">Defaults (override by adding below)</span>
          {Object.entries(defaults).map(([k, v]) => {
            const overridden = k in opts;
            return (
              <div key={k} className={`flex items-center gap-2 text-[10px] font-mono ${overridden ? "text-zinc-700 line-through" : "text-zinc-500"}`}>
                <span className="w-2.5" />
                <span>{k}</span>
                <span className="text-zinc-700">=</span>
                <span>{v}</span>
                {overridden && <span className="text-primary text-[9px] no-underline">overridden</span>}
              </div>
            );
          })}
        </div>

        {/* User overrides */}
        {overrideCount > 0 && (
          <div className="space-y-1">
            <span className="text-[9px] font-mono text-zinc-500 uppercase tracking-wider">Custom overrides</span>
            {Object.entries(opts).map(([k, v]) => (
              <div key={k} className="flex items-center gap-1.5">
                <span className="text-[10px] font-mono text-zinc-300 min-w-[140px]">{k}</span>
                <span className="text-[10px] text-zinc-700">=</span>
                <Input value={v} onChange={(e) => updateValue(k, e.target.value)}
                  className="h-6 text-[10px] font-mono flex-1" disabled={disabled} />
                {defaults[k] && <span className="text-[9px] font-mono text-zinc-700 shrink-0">was {defaults[k]}</span>}
                {!disabled && (
                  <button onClick={() => removeOption(k)} className="text-zinc-600 hover:text-red-400 shrink-0">
                    <X className="w-3 h-3" />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Add new */}
        {!disabled && (
          <div className="relative">
            <div className="flex items-center gap-1.5">
              <div className="relative flex-1">
                <Input value={draftKey}
                  onChange={(e) => { setDraftKey(e.target.value); setShowSuggestions(true); }}
                  onFocus={() => setShowSuggestions(true)}
                  onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
                  onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addOption(); } }}
                  placeholder="parameter name" className="h-6 text-[10px] font-mono" />
                {showSuggestions && draftKey && suggestions.length > 0 && (
                  <div className="absolute z-10 top-full mt-0.5 left-0 right-0 bg-zinc-900 border border-zinc-700 max-h-32 overflow-y-auto">
                    {suggestions.slice(0, 8).map((s) => (
                      <button key={s} type="button"
                        onMouseDown={(e) => { e.preventDefault(); setDraftKey(s); setDraftVal(defaults[s] || ""); setShowSuggestions(false); }}
                        className="w-full text-left px-2 py-1 text-[10px] font-mono text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200">
                        {s} <span className="text-zinc-600">= {defaults[s]}</span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <span className="text-[10px] text-zinc-700">=</span>
              <Input value={draftVal}
                onChange={(e) => setDraftVal(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addOption(); } }}
                placeholder="value" className="h-6 text-[10px] font-mono flex-1" />
              <Button variant="outline" size="sm" className="h-6 px-2" onClick={addOption}>
                <Plus className="w-3 h-3" />
              </Button>
            </div>
          </div>
        )}
      </div>
    </details>
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
      {/* Master + its config */}
      <MachineEditor label="Master" spec={topology.master} countLocked
        onChange={(s) => onChange({ ...topology, master: s })} disabled={disabled}>
        <OptionsEditor label="postgresql.conf" options={topology.master_options} locked={PG_MASTER_LOCKED} lockedHints={PG_MASTER_LOCKED_HINTS} defaults={PG_MASTER_DEFAULTS} disabled={disabled}
          onChange={(opts) => onChange({ ...topology, master_options: opts })} />
      </MachineEditor>

      {/* Replicas + their config */}
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
              }} disabled={disabled}>
              {i === 0 && (
                <OptionsEditor label="postgresql.conf (all replicas)" options={topology.replica_options} locked={PG_REPLICA_LOCKED} lockedHints={PG_REPLICA_LOCKED_HINTS} defaults={PG_REPLICA_DEFAULTS} disabled={disabled}
                  onChange={(opts) => onChange({ ...topology, replica_options: opts })} />
              )}
            </MachineEditor>
            {!disabled && (
              <button onClick={() => removeReplica(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      {/* HA toggles */}
      <div className="grid grid-cols-4 gap-2">
        <Toggle label="Patroni" checked={topology.patroni} onChange={(v) => onChange({ ...topology, patroni: v, ...(v ? { etcd: true } : {}) })} disabled={disabled} />
        <Toggle label="Etcd" checked={topology.etcd} onChange={(v) => onChange({ ...topology, etcd: v })} disabled={disabled} />
        <Toggle label="PgBouncer" checked={topology.pgbouncer} onChange={(v) => onChange({ ...topology, pgbouncer: v })} disabled={disabled} />
        <NumericSlider label="Sync Replicas" value={topology.sync_replicas} min={0} max={8}
          onChange={(v) => onChange({ ...topology, sync_replicas: v })} disabled={disabled} />
      </div>

      {/* Patroni config (inline when enabled) */}
      {topology.patroni && (
        <div className="border border-zinc-800/60 p-3 space-y-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Patroni</span>
          <OptionsEditor label="patroni.yml" options={topology.patroni_options} locked={PATRONI_LOCKED} lockedHints={PATRONI_LOCKED_HINTS} defaults={PATRONI_DEFAULTS} disabled={disabled}
            onChange={(opts) => onChange({ ...topology, patroni_options: opts })} />
        </div>
      )}

      {/* Etcd config (inline when enabled) */}
      {topology.etcd && (
        <div className="border border-zinc-800/60 p-3 space-y-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">Etcd</span>
          <OptionsEditor label="etcd config" options={topology.etcd_options} locked={ETCD_LOCKED} lockedHints={ETCD_LOCKED_HINTS} defaults={ETCD_DEFAULTS} disabled={disabled}
            onChange={(opts) => onChange({ ...topology, etcd_options: opts })} />
        </div>
      )}

      {/* PgBouncer config (inline when enabled) */}
      {topology.pgbouncer && (
        <div className="border border-zinc-800/60 p-3 space-y-2">
          <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">PgBouncer</span>
          <OptionsEditor label="pgbouncer.ini" options={topology.pgbouncer_options} locked={PGBOUNCER_LOCKED} lockedHints={PGBOUNCER_LOCKED_HINTS} defaults={PGBOUNCER_DEFAULTS} disabled={disabled}
            onChange={(opts) => onChange({ ...topology, pgbouncer_options: opts })} />
        </div>
      )}

      {/* HAProxy + config */}
      <div>
        <Toggle label="Enable HAProxy"
          checked={!!topology.haproxy}
          onChange={(v) => onChange({ ...topology, haproxy: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
          disabled={disabled} />
        {topology.haproxy && (
          <div className="mt-2">
            <MachineEditor label="HAProxy" spec={topology.haproxy}
              onChange={(s) => onChange({ ...topology, haproxy: s })} disabled={disabled}>
              <OptionsEditor label="haproxy.cfg" options={topology.haproxy_options} locked={HAPROXY_LOCKED} lockedHints={HAPROXY_LOCKED_HINTS} defaults={HAPROXY_DEFAULTS} disabled={disabled}
                onChange={(opts) => onChange({ ...topology, haproxy_options: opts })} />
            </MachineEditor>
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
      {/* Primary + config */}
      <MachineEditor label="Primary" spec={topology.primary} countLocked
        onChange={(s) => onChange({ ...topology, primary: s })} disabled={disabled}>
        <OptionsEditor label="my.cnf" options={topology.primary_options} locked={MYSQL_PRIMARY_LOCKED} lockedHints={MYSQL_PRIMARY_LOCKED_HINTS} defaults={MYSQL_PRIMARY_DEFAULTS} disabled={disabled}
          onChange={(opts) => onChange({ ...topology, primary_options: opts })} />
      </MachineEditor>

      {/* Replicas + config */}
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
              }} disabled={disabled}>
              {i === 0 && (
                <OptionsEditor label="my.cnf (all replicas)" options={topology.replica_options} locked={MYSQL_REPLICA_LOCKED} lockedHints={MYSQL_REPLICA_LOCKED_HINTS} defaults={MYSQL_REPLICA_DEFAULTS} disabled={disabled}
                  onChange={(opts) => onChange({ ...topology, replica_options: opts })} />
              )}
            </MachineEditor>
            {!disabled && (
              <button onClick={() => removeReplica(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      {/* Replication mode */}
      <div className="grid grid-cols-2 gap-2">
        <Toggle label="Group Replication" checked={topology.group_replication}
          onChange={(v) => onChange({ ...topology, group_replication: v, ...(v ? { semi_sync: false } : {}) })} disabled={disabled} />
        <Toggle label="Semi-Sync Replication" checked={topology.semi_sync}
          onChange={(v) => onChange({ ...topology, semi_sync: v, ...(v ? { group_replication: false } : {}) })} disabled={disabled} />
      </div>

      {/* ProxySQL + config */}
      <div>
        <Toggle label="Enable ProxySQL"
          checked={!!topology.proxysql}
          onChange={(v) => onChange({ ...topology, proxysql: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
          disabled={disabled} />
        {topology.proxysql && (
          <div className="mt-2">
            <MachineEditor label="ProxySQL" spec={topology.proxysql}
              onChange={(s) => onChange({ ...topology, proxysql: s })} disabled={disabled}>
              <OptionsEditor label="proxysql.cnf" options={topology.proxysql_options} locked={PROXYSQL_LOCKED} lockedHints={PROXYSQL_LOCKED_HINTS} defaults={PROXYSQL_DEFAULTS} disabled={disabled}
                onChange={(opts) => onChange({ ...topology, proxysql_options: opts })} />
            </MachineEditor>
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
      {/* Instances + config */}
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
              }} disabled={disabled}>
              {i === 0 && (
                <OptionsEditor label="picodata.yaml (all instances)" options={topology.instance_options} locked={PICO_LOCKED} lockedHints={PICO_LOCKED_HINTS} defaults={PICO_DEFAULTS} disabled={disabled}
                  onChange={(opts) => onChange({ ...topology, instance_options: opts })} />
              )}
            </MachineEditor>
            {!disabled && topology.instances.length > 1 && (
              <button onClick={() => removeInstance(i)} className="absolute top-2 right-2 text-zinc-600 hover:text-red-400">
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <NumericSlider label="Replication Factor" value={topology.replication_factor} min={1} max={5}
          onChange={(v) => onChange({ ...topology, replication_factor: v })} disabled={disabled} />
        <NumericSlider label="Shards" value={topology.shards} min={1} max={32}
          onChange={(v) => onChange({ ...topology, shards: v })} disabled={disabled} />
      </div>

      {/* HAProxy + config */}
      <div>
        <Toggle label="Enable HAProxy"
          checked={!!topology.haproxy}
          onChange={(v) => onChange({ ...topology, haproxy: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined })}
          disabled={disabled} />
        {topology.haproxy && (
          <div className="mt-2">
            <MachineEditor label="HAProxy" spec={topology.haproxy}
              onChange={(s) => onChange({ ...topology, haproxy: s })} disabled={disabled}>
              <OptionsEditor label="haproxy.cfg" options={topology.haproxy_options} locked={HAPROXY_LOCKED} lockedHints={HAPROXY_LOCKED_HINTS} defaults={PICO_HAPROXY_DEFAULTS} disabled={disabled}
                onChange={(opts) => onChange({ ...topology, haproxy_options: opts })} />
            </MachineEditor>
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
              <NumericSlider label="RF" value={tier.replication_factor} min={1} max={5}
                onChange={(v) => updateTier(i, { ...tier, replication_factor: v })} disabled={disabled} />
              <NumericSlider label="Count" value={tier.count} min={1} max={16}
                onChange={(v) => updateTier(i, { ...tier, count: v })} disabled={disabled} />
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

// ─── YDB Form ────────────────────────────────────────────────────

const YDB_STORAGE_DEFAULTS: Record<string, string> = { "--log-level": "WARN", "--grpc-port": "2135", "--mon-port": "8765", "--ic-port": "19001" };
const YDB_DATABASE_DEFAULTS: Record<string, string> = { "--log-level": "WARN", "--grpc-port": "2135", "--mon-port": "8766" };
const YDB_HAPROXY_DEFAULTS: Record<string, string> = { ...HAPROXY_DEFAULTS };

function YDBForm({ topology, onChange, disabled }: {
  topology: YDBTopology;
  onChange: (t: YDBTopology) => void;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-4">
      {/* Storage nodes */}
      <MachineEditor label="Storage Nodes" spec={topology.storage}
        onChange={(s) => onChange({ ...topology, storage: s })} disabled={disabled}>
        <OptionsEditor label="storage config" options={topology.storage_options}
          locked={{}} lockedHints={{}} defaults={YDB_STORAGE_DEFAULTS} disabled={disabled}
          onChange={(opts) => onChange({ ...topology, storage_options: opts })} />
      </MachineEditor>

      {/* Split mode (separate database/compute nodes) */}
      <div>
        <Toggle label="Separate database (compute) nodes"
          checked={!!topology.database}
          onChange={(v) => onChange({
            ...topology,
            database: v ? { role: "database" as const, count: 1, cpus: 2, memory_mb: 4096, disk_gb: 40 } : undefined,
          })}
          disabled={disabled} />
        {topology.database && (
          <div className="mt-2">
            <MachineEditor label="Database (Compute) Nodes" spec={topology.database}
              onChange={(s) => onChange({ ...topology, database: s })} disabled={disabled}>
              <OptionsEditor label="database config" options={topology.database_options}
                locked={{}} lockedHints={{}} defaults={YDB_DATABASE_DEFAULTS} disabled={disabled}
                onChange={(opts) => onChange({ ...topology, database_options: opts })} />
            </MachineEditor>
          </div>
        )}
      </div>

      {/* HAProxy */}
      <div>
        <Toggle label="HAProxy load balancer"
          checked={!!topology.haproxy}
          onChange={(v) => onChange({
            ...topology,
            haproxy: v ? { role: "proxy" as const, count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } : undefined,
          })}
          disabled={disabled} />
        {topology.haproxy && (
          <div className="mt-2">
            <MachineEditor label="HAProxy" spec={topology.haproxy}
              onChange={(s) => onChange({ ...topology, haproxy: s })} disabled={disabled}>
              <OptionsEditor label="haproxy.cfg" options={topology.haproxy_options}
                locked={HAPROXY_LOCKED} lockedHints={HAPROXY_LOCKED_HINTS} defaults={YDB_HAPROXY_DEFAULTS} disabled={disabled}
                onChange={(opts) => onChange({ ...topology, haproxy_options: opts })} />
            </MachineEditor>
          </div>
        )}
      </div>

      {/* Fault Tolerance + Database Path */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-[9px] font-mono text-zinc-600 uppercase tracking-wider">Fault Tolerance</Label>
          <Select value={topology.fault_tolerance}
            onValueChange={(v) => onChange({ ...topology, fault_tolerance: v })}
            disabled={disabled}>
            <SelectTrigger className="h-7 text-xs font-mono">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">none</SelectItem>
              <SelectItem value="block-4-2">block-4-2</SelectItem>
              <SelectItem value="mirror-3-dc">mirror-3-dc</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-[9px] font-mono text-zinc-600 uppercase tracking-wider">Database Path</Label>
          <Input value={topology.database_path}
            onChange={(e) => onChange({ ...topology, database_path: e.target.value })}
            className="h-7 text-xs font-mono" placeholder="/Root/testdb" disabled={disabled} />
        </div>
      </div>
    </div>
  );
}

// ─── Main Page ───────────────────────────────────────────────────

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres: { icon: Database, label: "PostgreSQL" },
  mysql:    { icon: Server,   label: "MySQL" },
  picodata: { icon: Cpu,      label: "Picodata" },
  ydb:      { icon: Database, label: "YDB" },
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
  const [ydbTopology, setYdbTopology] = useState<YDBTopology>(defaultYDB());

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
        else if (p.db_kind === "ydb") setYdbTopology(p.topology as YDBTopology);
      })
      .catch(() => setMessage({ type: "error", text: "Failed to load preset" }))
      .finally(() => setLoading(false));
  }, [id]);

  const currentTopology = useMemo(() => {
    if (dbKind === "postgres") return pgTopology;
    if (dbKind === "mysql") return myTopology;
    if (dbKind === "ydb") return ydbTopology;
    return picoTopology;
  }, [dbKind, pgTopology, myTopology, picoTopology, ydbTopology]);

  const errors = useMemo((): ValidationError[] => {
    const errs: ValidationError[] = [];
    if (!name.trim()) errs.push({ field: "name", message: "Name is required" });
    if (dbKind === "postgres") errs.push(...validatePostgres(pgTopology));
    else if (dbKind === "mysql") errs.push(...validateMySQL(myTopology));
    else if (dbKind === "picodata") errs.push(...validatePicodata(picoTopology));
    else if (dbKind === "ydb") errs.push(...validateYDB(ydbTopology));
    return errs;
  }, [name, dbKind, pgTopology, myTopology, picoTopology, ydbTopology]);

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
          {/* Database kind selector */}
          <div>
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Database</Label>
            <div className="grid grid-cols-4 gap-2">
              {ALL_DB_KINDS.map((k) => {
                const meta = DB_META[k];
                const kColor = DB_COLORS[k];
                const Icon = meta.icon;
                const active = dbKind === k;
                return (
                  <button key={k} type="button"
                    onClick={() => !isEdit && !isBuiltin && setDbKind(k)}
                    className={`flex items-center gap-2 border p-2.5 transition-all ${
                      active ? `${kColor.accent}` : "border-zinc-800/60 hover:border-zinc-700"
                    } ${isEdit || isBuiltin ? "cursor-not-allowed opacity-60" : "cursor-pointer"}`}
                  >
                    <Icon className={`h-4 w-4 ${active ? kColor.text : "text-zinc-600"}`} />
                    <span className={`text-sm font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>
                      {meta.label}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Name / Description */}
          <div className="grid grid-cols-2 gap-3">
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
            {dbKind === "ydb" && (
              <YDBForm topology={ydbTopology} onChange={setYdbTopology} disabled={isBuiltin} />
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
