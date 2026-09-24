// GENERATED from schemapb schema workload.segment@1 — do not edit.
// One stroppy load segment: workload, typed parameters, scenario, steps and thresholds.

/** variant baseline of workload */
export interface WorkloadSegment1WorkloadBaseline {
  script: "baseline";
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Rows. Rows loaded into the baseline probe table (rows). */
  rows?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
}

/** variant execute_sql of workload */
export interface WorkloadSegment1WorkloadExecuteSql {
  script: "execute_sql";
  /** Inline SQL. SQL text to execute (sqlBody); may start with a `--= name` marker to name the query. */
  sql_body?: string | null;
  /** SQL file. SQL file to execute (sqlFile): a preset id shipped with stroppy; use sql_body for inline SQL. */
  sql_file?: string | null;
}

/** variant simple of workload */
export interface WorkloadSegment1WorkloadSimple {
  script: "simple";
}

/** variant tpcb/procs of workload */
export interface WorkloadSegment1WorkloadTpcbProcs {
  script: "tpcb/procs";
  /** Scale factor. TPC-B scale factor = branches (scaleFactor); 100k accounts per branch. */
  scale_factor?: number | string;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcb/pico. */
  sql_file?: string | null;
}

/** variant tpcb/tx of workload */
export interface WorkloadSegment1WorkloadTpcbTx {
  script: "tpcb/tx";
  /** Scale factor. TPC-B scale factor = branches (scaleFactor); 100k accounts per branch. */
  scale_factor?: number | string;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcb/pico. */
  sql_file?: string | null;
}

/** variant tpcc/procs of workload */
export interface WorkloadSegment1WorkloadTpccProcs {
  script: "tpcc/procs";
  /** Warehouses. Number of warehouses (scaleFactor); ~100 MB per warehouse drives the disk requirement. */
  scale_factor?: number | string;
  /** First warehouse. First warehouse id (warehouseStart); lets several runners share one database. */
  warehouse_start?: number | string;
  /** Load items. Load the shared item table (loadItems); unset = only when warehouse_start is 1. */
  load_items?: boolean | null;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Unlogged tables while loading. Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only. */
  pg_unlogged?: boolean;
  /** Pacing. Apply TPC-C keying and think times (pacing); needed for a compliance verdict. */
  pacing?: boolean;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcc/ydb_no_indexes. */
  sql_file?: string | null;
}

/** variant tpcc/tx of workload */
export interface WorkloadSegment1WorkloadTpccTx {
  script: "tpcc/tx";
  /** Warehouses. Number of warehouses (scaleFactor); ~100 MB per warehouse drives the disk requirement. */
  scale_factor?: number | string;
  /** First warehouse. First warehouse id (warehouseStart); lets several runners share one database. */
  warehouse_start?: number | string;
  /** Load items. Load the shared item table (loadItems); unset = only when warehouse_start is 1. */
  load_items?: boolean | null;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Unlogged tables while loading. Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only. */
  pg_unlogged?: boolean;
  /** Pacing. Apply TPC-C keying and think times (pacing); needed for a compliance verdict. */
  pacing?: boolean;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcc/ydb_no_indexes. */
  sql_file?: string | null;
}

/** variant tpcds of workload */
export interface WorkloadSegment1WorkloadTpcds {
  script: "tpcds";
  /** Scale factor. TPC-DS scale factor (scaleFactor); fractional allowed. Static dimensions (~1.9M customer_demographics rows) do not shrink. */
  scale_factor?: number;
  /** Load workers. Workers used to load each table (loadWorkers); 0 = automatic. */
  load_workers?: number | string;
  /** Unlogged tables while loading. Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only. */
  pg_unlogged?: boolean;
  /** Query streams. Number of query streams (streams). */
  streams?: number | string;
  /** Query stream. Generated query stream (queryStream); unset uses the baked query set. Explicit zero selects generated stream 0. */
  query_stream?: number | string | null;
  /** Query seed. Query generator seed (querySeed). */
  query_seed?: number | string;
  /** Validate outside SF=1. Compare answers even when the scale factor is not 1 (validateForce). */
  validate_force?: boolean;
  /** YDB store mode. YDB table store mode (ydbStoreMode); ignored by other drivers. */
  ydb_store_mode?: "column" | "row";
  /** Schema file. Schema SQL override (schemaFile): a preset id shipped with stroppy, like tpcds/schema.pico. */
  schema_file?: string | null;
  /** SQL file. Query SQL override (sqlFile): a preset id shipped with stroppy, like tpcds/pico. */
  sql_file?: string | null;
}

/** variant tpch/tx of workload */
export interface WorkloadSegment1WorkloadTpchTx {
  script: "tpch/tx";
  /** Scale factor. TPC-H scale factor (scaleFactor); fractional allowed, 1 ≈ 1 GB of data. */
  scale_factor?: number;
  /** Load workers. Workers used to load each table (loadWorkers); 0 = automatic. */
  load_workers?: number | string;
  /** Unlogged tables while loading. Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only. */
  pg_unlogged?: boolean;
  /** YDB store mode. YDB table store mode (ydbStoreMode); ignored by other drivers. */
  ydb_store_mode?: "column" | "row";
  /** SQL file. Dialect file override (sqlFile): a preset id shipped with stroppy, like tpch/pico. */
  sql_file?: string | null;
}

/** object run */
export interface WorkloadSegment1Run {
  /** Executor. Scenario executor: constant-vus runs for a duration, shared-iterations shares N iterations between VUs. */
  executor?: "constant-vus" | "shared-iterations";
  /** Virtual users. Concurrent virtual users (vus); sizes the runner machine and the pool. */
  vus?: number | string;
  /** Duration. Wall-clock length of a constant-vus scenario (duration). */
  duration?: string | null;
  /** Iterations. Total iterations of a shared-iterations scenario (iterations). */
  iterations?: number | string | null;
  /** Query timeout. Per-statement deadline (queryTimeout); 0 disables it. */
  query_timeout?: string;
}

/** object thresholds */
export interface WorkloadSegment1Thresholds {
  /** p99 latency. Fail the segment when iteration_duration p99 exceeds this. [ms] */
  p99_ms?: number;
  /** Error rate. Fail the segment when failed_iterations / iterations exceeds this (0..1). [ratio] */
  error_rate?: number;
}

/** root */
export interface WorkloadSegment1 {
  /** Name. Segment id inside the workload; used as the phase label of the run and as a metric label. */
  name: string;
  /** Workload. Built-in stroppy workload and its typed parameters; `script` is the id passed to `stroppy run`. */
  workload: WorkloadSegment1WorkloadBaseline | WorkloadSegment1WorkloadExecuteSql | WorkloadSegment1WorkloadSimple | WorkloadSegment1WorkloadTpcbProcs | WorkloadSegment1WorkloadTpcbTx | WorkloadSegment1WorkloadTpccProcs | WorkloadSegment1WorkloadTpccTx | WorkloadSegment1WorkloadTpcds | WorkloadSegment1WorkloadTpchTx;
  /** Scenario. Load shape of the segment. */
  run: WorkloadSegment1Run;
  /** Only these steps. Run only the listed stroppy steps (--steps); empty = all steps. */
  steps?: Array<string>;
  /** Skip these steps. Skip the listed stroppy steps (--no-steps); stroppy rejects it together with steps. */
  no_steps?: Array<string>;
  /** Extra parameters. Typed stroppy flags this form does not model, by flag name without dashes (load-workers); values are parsed by stroppy. */
  extra_params?: Record<string, string>;
  /** Thresholds. Pass/fail bounds the pipeline applies to the segment summary. */
  thresholds?: WorkloadSegment1Thresholds;
  /** Random seed. Stroppy global.seed; zero uses Stroppy's random seed, a positive value makes generation reproducible. */
  seed?: number | string | null;
  /** Segment timeout. Execution deadline after the warmup wait, including container preparation and data load; unset uses duration plus headroom, or 24h for iterations. */
  timeout?: string | null;
  /** Warm-up. Idle wait before the segment starts, letting caches and replicas settle. */
  warmup?: string;
  /** Log level. Minimum stroppy log level (--log-level); debug traces parameter resolution. */
  log_level?: "debug" | "info" | "warn" | "error";
}
