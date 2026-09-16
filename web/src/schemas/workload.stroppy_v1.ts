// GENERATED from schemapb schema workload.stroppy@1 — do not edit.
// A stroppy workload: build, protocol, load segments, driver options and baseline.

/** variant baseline of workload */
export interface WorkloadStroppy1SegmentWorkloadBaseline {
  script: "baseline";
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Rows. Rows loaded into the baseline probe table (rows). */
  rows?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
}

/** variant execute_sql of workload */
export interface WorkloadStroppy1SegmentWorkloadExecuteSql {
  script: "execute_sql";
  /** Inline SQL. SQL text to execute (sqlBody); may start with a `--= name` marker to name the query. */
  sql_body?: string | null;
  /** SQL file. SQL file to execute (sqlFile): a file shipped in files. */
  sql_file?: string | null;
}

/** variant simple of workload */
export interface WorkloadStroppy1SegmentWorkloadSimple {
  script: "simple";
}

/** variant tpcb/procs of workload */
export interface WorkloadStroppy1SegmentWorkloadTpcbProcs {
  script: "tpcb/procs";
  /** Scale factor. TPC-B scale factor = branches (scaleFactor); 100k accounts per branch. */
  scale_factor?: number | string;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id like tpcb/pico or a file shipped in files. */
  sql_file?: string | null;
}

/** variant tpcb/tx of workload */
export interface WorkloadStroppy1SegmentWorkloadTpcbTx {
  script: "tpcb/tx";
  /** Scale factor. TPC-B scale factor = branches (scaleFactor); 100k accounts per branch. */
  scale_factor?: number | string;
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Retry attempts. Maximum attempts of one transaction before the iteration fails (retryAttempts). */
  retry_attempts?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
  /** SQL file. Dialect file override (sqlFile): a preset id like tpcb/pico or a file shipped in files. */
  sql_file?: string | null;
}

/** variant tpcc/procs of workload */
export interface WorkloadStroppy1SegmentWorkloadTpccProcs {
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
  /** SQL file. Dialect file override (sqlFile): a preset id like tpcc/ydb_no_indexes or a file shipped in files. */
  sql_file?: string | null;
}

/** variant tpcc/tx of workload */
export interface WorkloadStroppy1SegmentWorkloadTpccTx {
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
  /** SQL file. Dialect file override (sqlFile): a preset id like tpcc/ydb_no_indexes or a file shipped in files. */
  sql_file?: string | null;
}

/** variant tpcds of workload */
export interface WorkloadStroppy1SegmentWorkloadTpcds {
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
  /** Schema file. Schema SQL override (schemaFile): a preset id like tpcds/schema.pico or a file shipped in files. */
  schema_file?: string | null;
  /** SQL file. Query SQL override (sqlFile): a preset id like tpcds/pico or a file shipped in files. */
  sql_file?: string | null;
}

/** variant tpch/tx of workload */
export interface WorkloadStroppy1SegmentWorkloadTpchTx {
  script: "tpch/tx";
  /** Scale factor. TPC-H scale factor (scaleFactor); fractional allowed, 1 ≈ 1 GB of data. */
  scale_factor?: number;
  /** Load workers. Workers used to load each table (loadWorkers); 0 = automatic. */
  load_workers?: number | string;
  /** Unlogged tables while loading. Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only. */
  pg_unlogged?: boolean;
  /** YDB store mode. YDB table store mode (ydbStoreMode); ignored by other drivers. */
  ydb_store_mode?: "column" | "row";
  /** SQL file. Dialect file override (sqlFile): a preset id like tpch/pico or a file shipped in files. */
  sql_file?: string | null;
}

/** object run */
export interface WorkloadStroppy1SegmentRun {
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

/** object  */
export interface WorkloadStroppy1SegmentItem {
  /** File name. Name the file gets in the segment workspace; reference it from sql_file/schema_file. */
  name: string;
  /** Kind. What the file is: a schema/DDL file, a config, or a data file. */
  kind?: "sql" | "conf" | "data";
  /** Content. Inline file body, at most 1 MiB. */
  content?: string;
  /** Reference. Graphene artifact reference in the current namespace, artifact/<name>; fetched on the runner. */
  ref?: string;
}

/** object thresholds */
export interface WorkloadStroppy1SegmentThresholds {
  /** p99 latency. Fail the segment when iteration_duration p99 exceeds this. [ms] */
  p99_ms?: number;
  /** Error rate. Fail the segment when failed_iterations / iterations exceeds this (0..1). [ratio] */
  error_rate?: number;
}

/** def segment */
export interface WorkloadStroppy1Segment {
  /** Name. Segment id inside the workload; used as the phase label of the run and as a metric label. */
  name: string;
  /** Workload. Built-in stroppy workload and its typed parameters; `script` is the id passed to `stroppy run`. */
  workload: WorkloadStroppy1SegmentWorkloadBaseline | WorkloadStroppy1SegmentWorkloadExecuteSql | WorkloadStroppy1SegmentWorkloadSimple | WorkloadStroppy1SegmentWorkloadTpcbProcs | WorkloadStroppy1SegmentWorkloadTpcbTx | WorkloadStroppy1SegmentWorkloadTpccProcs | WorkloadStroppy1SegmentWorkloadTpccTx | WorkloadStroppy1SegmentWorkloadTpcds | WorkloadStroppy1SegmentWorkloadTpchTx;
  /** Scenario. Load shape of the segment. */
  run: WorkloadStroppy1SegmentRun;
  /** Only these steps. Run only the listed stroppy steps (--steps); empty = all steps. */
  steps?: Array<string>;
  /** Skip these steps. Skip the listed stroppy steps (--no-steps); stroppy rejects it together with steps. */
  no_steps?: Array<string>;
  /** Extra parameters. Typed stroppy flags this form does not model, by flag name without dashes (load-workers); values are parsed by stroppy. */
  extra_params?: Record<string, string>;
  /** Files. Extra files (SQL dialects, schemas, data) shipped with the segment. */
  files?: Array<WorkloadStroppy1SegmentItem>;
  /** Thresholds. Pass/fail bounds the pipeline applies to the segment summary. */
  thresholds?: WorkloadStroppy1SegmentThresholds;
  /** Random seed. Stroppy global.seed; zero uses Stroppy's random seed, a positive value makes generation reproducible. */
  seed?: number | string | null;
  /** Segment timeout. Execution deadline after the warmup wait, including container preparation and data load; unset uses duration plus headroom, or 24h for iterations. */
  timeout?: string | null;
  /** Warm-up. Idle wait before the segment starts, letting caches and replicas settle. */
  warmup?: string;
  /** Log level. Minimum stroppy log level (--log-level); debug traces parameter resolution. */
  log_level?: "debug" | "info" | "warn" | "error";
}

/** object pool */
export interface WorkloadStroppy1DriverPool {
  /** Max connections. Stroppy max_conns; unset preserves the driver default. */
  max_conns?: number | string | null;
  /** Min connections. Stroppy min_conns; unset preserves the driver default. */
  min_conns?: number | string | null;
  /** Min idle connections. Stroppy min_idle_conns; unset preserves the driver default. */
  min_idle_conns?: number | string | null;
  /** Max lifetime. Stroppy max_conn_lifetime; zero disables the lifetime limit. */
  max_conn_lifetime?: string | null;
  /** Max idle time. Stroppy max_conn_idle_time; zero disables the lifetime limit. */
  max_conn_idle_time?: string | null;
  /** Description cache. Stroppy description_cache_capacity; unset preserves the driver default. */
  description_cache_capacity?: number | string | null;
  /** Statement cache. Stroppy statement_cache_capacity; unset preserves the driver default. */
  statement_cache_capacity?: number | string | null;
  /** Driver log level. pgx tracer log level (traceLogLevel). */
  trace_log_level?: "trace" | "debug" | "info" | "warn" | "error" | "none" | null;
  /** Query execution mode. pgx defaultQueryExecMode. */
  default_query_exec_mode?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol" | null;
  /** Max open connections. Stroppy max_open_conns; unset preserves the driver default. */
  max_open_conns?: number | string | null;
  /** Max idle connections. Stroppy max_idle_conns; unset preserves the driver default. */
  max_idle_conns?: number | string | null;
  /** Max lifetime. Stroppy conn_max_lifetime; zero disables the lifetime limit. */
  conn_max_lifetime?: string | null;
  /** Max idle time. Stroppy conn_max_idle_time; zero disables the lifetime limit. */
  conn_max_idle_time?: string | null;
}

/** object postgres */
export interface WorkloadStroppy1DriverPostgres {
  /** Max connections. Stroppy max_conns; unset preserves the driver default. */
  max_conns?: number | string | null;
  /** Min connections. Stroppy min_conns; unset preserves the driver default. */
  min_conns?: number | string | null;
  /** Min idle connections. Stroppy min_idle_conns; unset preserves the driver default. */
  min_idle_conns?: number | string | null;
  /** Max lifetime. Stroppy max_conn_lifetime; zero disables the lifetime limit. */
  max_conn_lifetime?: string | null;
  /** Max idle time. Stroppy max_conn_idle_time; zero disables the lifetime limit. */
  max_conn_idle_time?: string | null;
  /** Description cache. Stroppy description_cache_capacity; unset preserves the driver default. */
  description_cache_capacity?: number | string | null;
  /** Statement cache. Stroppy statement_cache_capacity; unset preserves the driver default. */
  statement_cache_capacity?: number | string | null;
  /** Driver log level. pgx tracer log level (traceLogLevel). */
  trace_log_level?: "trace" | "debug" | "info" | "warn" | "error" | "none" | null;
  /** Query execution mode. pgx defaultQueryExecMode. */
  default_query_exec_mode?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol" | null;
}

/** object sql */
export interface WorkloadStroppy1DriverSql {
  /** Max open connections. Stroppy max_open_conns; unset preserves the driver default. */
  max_open_conns?: number | string | null;
  /** Max idle connections. Stroppy max_idle_conns; unset preserves the driver default. */
  max_idle_conns?: number | string | null;
  /** Max lifetime. Stroppy conn_max_lifetime; zero disables the lifetime limit. */
  conn_max_lifetime?: string | null;
  /** Max idle time. Stroppy conn_max_idle_time; zero disables the lifetime limit. */
  conn_max_idle_time?: string | null;
}

/** object insert_progress */
export interface WorkloadStroppy1DriverInsertProgress {
  /** Enabled. Explicit insertProgress.enabled override; unset uses the mode. */
  enabled?: boolean | null;
  /** Mode. insertProgress.mode: where load progress goes. */
  mode?: "off" | "log" | "metrics" | "both";
  /** Interval. insertProgress.interval — progress cadence. */
  interval?: string;
  /** Stall after. insertProgress.stallAfter — warn when no rows moved for this long. */
  stall_after?: string;
}

/** object driver */
export interface WorkloadStroppy1Driver {
  /** Insert method. Fallback for load requests that leave their method unset (defaultInsertMethod); workloads normally choose themselves. */
  default_insert_method?: "native" | "columnar" | "plain_bulk" | "plain_query" | null;
  /** Bulk size. Rows per bulk INSERT statement (bulkSize). [rows] */
  bulk_size?: number | string;
  /** Connection pool. Native Stroppy pool options; explicit driver settings take precedence over pool aliases. */
  pool?: WorkloadStroppy1DriverPool;
  /** PostgreSQL driver. Native Stroppy postgres options; explicit driver settings take precedence over pool aliases. */
  postgres?: WorkloadStroppy1DriverPostgres;
  /** SQL driver. Native Stroppy sql options; explicit driver settings take precedence over pool aliases. */
  sql?: WorkloadStroppy1DriverSql;
  /** Load progress. insertProgress.* — load progress reporting. */
  insert_progress?: WorkloadStroppy1DriverInsertProgress;
}

/** variant cockroach of connection */
export interface WorkloadStroppy1ConnectionCockroach {
  kind: "cockroach";
  /** SSL mode. TLS negotiation mode of the cockroach (pgwire) connection. */
  sslmode?: "disable" | "require" | "verify-full";
  /** Application name. */
  application_name?: string;
}

/** variant mysql of connection */
export interface WorkloadStroppy1ConnectionMysql {
  kind: "mysql";
  /** TLS. go-sql-driver TLS mode of the MySQL DSN. */
  tls?: "false" | "preferred" | "skip-verify" | "true";
  /** Charset. Connection character set. */
  charset?: string;
}

/** variant noop of connection */
export interface WorkloadStroppy1ConnectionNoop {
  kind: "noop";
}

/** variant pg of connection */
export interface WorkloadStroppy1ConnectionPg {
  kind: "pg";
  /** SSL mode. TLS negotiation mode of the postgres connection (URL sslmode). */
  sslmode?: "disable" | "require" | "verify-full";
  /** Application name. application_name reported to postgres; shows up in pg_stat_activity. */
  application_name?: string;
  /** Query exec mode. pgx query execution mode (postgres.defaultQueryExecMode). */
  query_exec_mode?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol" | null;
}

/** variant picodata of connection */
export interface WorkloadStroppy1ConnectionPicodata {
  kind: "picodata";
  /** Query execution mode. pgx execution mode. Exec avoids prepared-statement limits and binary parameter incompatibilities in Picodata 25.3 and 26.1. */
  query_exec_mode?: "exec" | "cache_statement" | "cache_describe" | "describe_exec" | null;
}

/** variant ydb of connection */
export interface WorkloadStroppy1ConnectionYdb {
  kind: "ydb";
  /** CA certificate. PEM of a private CA (caCertFile), when the endpoint is not signed by a public one. */
  ca_cert?: string | null;
  /** Auth token. IAM token passed as authToken. */
  auth_token?: string | null;
  /** User. Static credentials user (authUser). */
  auth_user?: string | null;
  /** Password. Static credentials password (authPassword). */
  auth_password?: string | null;
  /** Skip TLS verification. tlsInsecureSkipVerify — testing only. */
  tls_insecure_skip_verify?: boolean;
}

/** object baseline */
export interface WorkloadStroppy1Baseline {
  /** Measure the runner. Run `stroppy baseline` on the runner machine before the segments: the stroppy ceiling a database run can never exceed. */
  enabled?: boolean;
  /** Tiers. Which tiers to run (--tiers); unset = both. */
  tiers?: Array<"noop" | "wire">;
  /** Quick. Shorter phases and a smaller load (--quick). */
  quick?: boolean;
  /** Parallel VUs. VU count of the parallel tx phase (--vus); unset = 20. */
  vus?: number | string | null;
  /** Load rows. Rows loaded into the probe table (--rows); unset = 250000. */
  rows?: number | string | null;
  /** Tx phase duration. Duration of each tx phase (--duration); unset = 3s. */
  duration?: string | null;
}

/** root */
export interface WorkloadStroppy1 {
  /** Stroppy version. Stroppy build to run: a release (6.0.0) or a nightly of one commit (nightly-<sha>); must exist in the platform catalog. 6.0.0 is the minimum. */
  stroppy_version: string;
  /** Protocol. Wire protocol stroppy talks to the database with. */
  protocol: "pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop";
  /** Segments. Ordered stroppy invocations; each one is a phase of the run. */
  segments: Array<WorkloadStroppy1Segment>;
  /** Driver. Protocol-neutral stroppy driver options. */
  driver?: WorkloadStroppy1Driver;
  /** Connection. Protocol-specific connection options; the kind must match the protocol. */
  connection?: WorkloadStroppy1ConnectionCockroach | WorkloadStroppy1ConnectionMysql | WorkloadStroppy1ConnectionNoop | WorkloadStroppy1ConnectionPg | WorkloadStroppy1ConnectionPicodata | WorkloadStroppy1ConnectionYdb;
  /** Baseline. Machine self-check with `stroppy baseline`; its JSON report lands in the run result. */
  baseline?: WorkloadStroppy1Baseline;
}
