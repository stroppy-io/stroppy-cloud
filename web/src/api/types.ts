// --- Enums / constants ---

export type Provider = "yandex" | "docker";
export type DatabaseKind = "postgres" | "mysql" | "picodata";

export type Phase =
  | "network"
  | "machines"
  | "install_db"
  | "configure_db"
  | "install_monitor"
  | "configure_monitor"
  | "install_stroppy"
  | "run_stroppy"
  | "install_etcd"
  | "configure_etcd"
  | "install_proxy"
  | "configure_proxy"
  | "install_pgbouncer"
  | "configure_pgbouncer"
  | "teardown";

export type MachineRole =
  | "database"
  | "monitor"
  | "stroppy"
  | "etcd"
  | "proxy"
  | "pgbouncer";

export type NodeStatusValue = "pending" | "done" | "failed";

// --- Machine / Topology ---

export interface MachineSpec {
  role: MachineRole;
  count: number;
  cpus: number;
  memory_mb: number;
  disk_gb: number;
}

export interface PostgresTopology {
  master: MachineSpec;
  replicas?: MachineSpec[];
  haproxy?: MachineSpec;
  pgbouncer: boolean;
  patroni: boolean;
  etcd: boolean;
  sync_replicas: number;
  options?: Record<string, string>;
}

export interface MySQLTopology {
  primary: MachineSpec;
  replicas?: MachineSpec[];
  proxysql?: MachineSpec;
  group_replication: boolean;
  semi_sync: boolean;
  options?: Record<string, string>;
}

export interface PicodataTier {
  name: string;
  replication_factor: number;
  can_vote: boolean;
  count: number;
}

export interface PicodataTopology {
  instances: MachineSpec[];
  haproxy?: MachineSpec;
  replication_factor: number;
  shards: number;
  tiers?: PicodataTier[];
  options?: Record<string, string>;
}

export interface DatabaseConfig {
  kind: DatabaseKind;
  version: string;
  postgres?: PostgresTopology;
  mysql?: MySQLTopology;
  picodata?: PicodataTopology;
}

export interface MonitorConfig {
  metrics_endpoint?: string;
  logs_endpoint?: string;
}

export interface StroppyConfig {
  version: string;
  workload: string;
  duration: string;
  vus_scale?: number;
  pool_size?: number;
  scale_factor?: number;
  workers?: number; // deprecated
  options?: Record<string, string>;
}

export interface NetworkConfig {
  cidr: string;
  zone?: string;
}

// Package is a first-class entity stored in the DB.
export interface Package {
  id: string;
  name: string;
  description: string;
  db_kind: string;
  db_version: string;
  is_builtin: boolean;
  apt_packages: string[];
  pre_install: string[];
  custom_repo?: string;
  custom_repo_key?: string;
  deb_filename?: string;
  has_deb?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface RunConfig {
  id: string;
  provider: Provider;
  network: NetworkConfig;
  machines: MachineSpec[];
  database: DatabaseConfig;
  monitor: MonitorConfig;
  stroppy: StroppyConfig;
  preset_id?: string;
  package_id?: string;
}

// --- DAG / Snapshot ---

export interface NodeStatus {
  id: string;
  status: NodeStatusValue;
  error?: string;
}

export interface Snapshot {
  graph: string; // JSON-encoded graph
  nodes: NodeStatus[];
  started_at?: string;
  finished_at?: string;
  state?: {
    provider?: string;
    run_config?: Record<string, unknown> | string; // object or JSON string
  };
}

// --- Run Summary ---

export interface RunSummary {
  id: string;
  nodes: NodeStatus[];
  total: number;
  done: number;
  failed: number;
  pending: number;
  started_at?: string;
  finished_at?: string;
  db_kind?: string;
  provider?: string;
}

// --- Presets ---

// Legacy format (kept for backward compatibility with TopologyDiagram).
export interface PresetsResponse {
  postgres: Record<string, PostgresTopology>;
  mysql: Record<string, MySQLTopology>;
  picodata: Record<string, PicodataTopology>;
}

// Per-tenant preset stored in the DB.
export interface Preset {
  id: string;
  name: string;
  description: string;
  db_kind: DatabaseKind;
  is_builtin: boolean;
  topology: PostgresTopology | MySQLTopology | PicodataTopology;
  created_at?: string;
}

// --- Settings ---

export interface YandexCloudSettings {
  token: string;
  cloud_id: string;
  folder_id: string;
  zone: string;
  network_id: string;
  network_name: string;
  subnet_cidr: string;
  platform_id: string;
  image_id: string;
  assign_public_ip: boolean;
  ssh_user: string;
  ssh_public_key: string;
}

export interface CloudSettings {
  yandex: YandexCloudSettings;
  server_addr: string;
  binary_url: string;
}

export interface GrafanaSettings {
  url: string;
  embed_enabled: boolean;
  dashboards: Record<string, string>;
}

export interface ServerSettings {
  cloud: CloudSettings;
  webhooks?: Record<string, unknown>;
}

// --- Metrics / Compare ---

export interface MetricValue {
  name: string;
  value: number;
  unit: string;
}

export interface ComparisonRow {
  key: string;
  name: string;
  unit: string;
  avg_a: number;
  avg_b: number;
  max_a: number;
  max_b: number;
  diff_avg_pct: number;
  diff_max_pct: number;
  verdict: "better" | "worse" | "same";
}

export interface ComparisonResponse {
  run_a: string;
  run_b: string;
  start: string;
  end: string;
  metrics: ComparisonRow[];
  summary: { better: number; worse: number; same: number };
}

// --- Auth / Multi-tenancy ---

export interface AuthUser {
  id: string;
  username: string;
  tenant_id: string | null;
  tenant_name: string | null;
  role: "viewer" | "operator" | "owner";
  is_root: boolean;
  tenants: { id: string; tenant_name: string; role: string }[];
}

export interface Tenant {
  id: string;
  name: string;
  created_at: string;
}

export interface TenantMember {
  tenant_id: string;
  user_id: string;
  username: string;
  role: string;
  created_at: string;
}

export interface TenantAPIToken {
  id: string;
  tenant_id: string;
  name: string;
  role: string;
  created_by: string;
  expires_at: string | null;
  created_at: string;
}

// --- WebSocket messages ---

export interface WSMessage {
  type: "log" | "report" | "agent_log";
  run_id?: string;
  node_id?: string;
  payload: unknown;
}

export interface LogLine {
  run_id: string;
  phase: string;
  machine_id: string;
  line: string;
  ts: string;
}
