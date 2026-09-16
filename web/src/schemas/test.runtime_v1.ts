// GENERATED from schemapb schema test.runtime@1 — do not edit.

/** variant create of network */
export interface TestRuntime1AwsSettingsNetworkCreate {
  kind: "create";
  /** VPC CIDR. IPv4 range of the VPC created for the run. */
  cidr?: string;
}

/** variant existing of network */
export interface TestRuntime1AwsSettingsNetworkExisting {
  kind: "existing";
  /** VPC id. Existing VPC the machines join. */
  vpc_id: string;
  /** Subnet id. Existing subnet in the chosen availability zone. */
  subnet_id: string;
  /** Security group id. Existing security group applied to every machine. */
  security_group_id?: string;
}

/** def aws_settings */
export interface TestRuntime1AwsSettings {
  /** Region. AWS commercial region the run is created in; opt-in regions must be enabled on the account. */
  region: "us-east-1" | "us-east-2" | "us-west-1" | "us-west-2" | "af-south-1" | "ap-east-1" | "ap-east-2" | "ap-south-1" | "ap-south-2" | "ap-northeast-1" | "ap-northeast-2" | "ap-northeast-3" | "ap-southeast-1" | "ap-southeast-2" | "ap-southeast-3" | "ap-southeast-4" | "ap-southeast-5" | "ap-southeast-6" | "ap-southeast-7" | "ca-central-1" | "ca-west-1" | "eu-central-1" | "eu-central-2" | "eu-west-1" | "eu-west-2" | "eu-west-3" | "eu-north-1" | "eu-south-1" | "eu-south-2" | "il-central-1" | "mx-central-1" | "me-south-1" | "me-central-1" | "sa-east-1";
  /** Availability zone. Availability zone inside the region; empty lets AWS pick one. */
  availability_zone?: string;
  /** Network. Create a throwaway VPC for every run, or place runs into an existing one. */
  network?: TestRuntime1AwsSettingsNetworkCreate | TestRuntime1AwsSettingsNetworkExisting;
  /** Instance family. EC2 family the size table resolves instance types in. */
  instance_family?: "m6i" | "m7i" | "m6a" | "m7a" | "c6i" | "c7i" | "r6i" | "r7i";
  /** AMI family. Boot image family; the concrete AMI id is resolved per region at launch. */
  ami_family?: "ubuntu-24.04" | "ubuntu-22.04" | "al2023";
  /** Public IPs. Give every machine a public IPv4 address. */
  public_ips?: boolean;
  /** Spot instances. Use Spot capacity: much cheaper, but interruptible with a two-minute notice — do not use for a measurement that must complete. */
  spot?: boolean;
}

/** def baseline */
export interface TestRuntime1Baseline {
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

/** object  */
export interface TestRuntime1ContainerItem {
  /** Container port. */
  container: number | string;
  /** Host port. Host-network port; remapping is not supported. */
  host: number | string;
}

/** object  */
export interface TestRuntime1ContainerItem2 {
  /** Host path. */
  source: string;
  /** Container path. */
  target: string;
  /** Read-only. */
  ro?: boolean;
}

/** object  */
export interface TestRuntime1ContainerItem3 {
  /** Path. Absolute path inside the container. */
  path: string;
  /** Content. Rendered config text (cfg.* schema Render output). */
  content: string;
  /** Mode. Octal file mode. */
  mode?: string;
}

/** object healthcheck */
export interface TestRuntime1ContainerHealthcheck {
  /** Command. */
  cmd: Array<string>;
  /** Interval. */
  interval?: string;
  /** Retries. */
  retries?: number | string;
}

/** def container */
export interface TestRuntime1Container {
  /** Image. Fully qualified docker image; every database is a container, no host packages. */
  image?: string;
  /** Command. Argv replacing the image command. */
  cmd?: Array<string>;
  /** Entrypoint. Executable and arguments replacing the image entrypoint; omit to use the upstream image entrypoint. */
  entrypoint?: Array<string>;
  /** Environment. */
  env?: Record<string, string>;
  /** Ports. */
  ports?: Array<TestRuntime1ContainerItem>;
  /** Mounts. */
  mounts?: Array<TestRuntime1ContainerItem2>;
  /** Files. Configs rendered by the server and injected at start. */
  files?: Array<TestRuntime1ContainerItem3>;
  /** Scrape path. Prometheus metrics path on the container, e.g. /metrics; empty = not scraped. */
  scrape?: string;
  /** Metrics port. HTTP port for the scrape path; when omitted, uses the first container port for compatibility. */
  scrape_port?: number | string;
  /** Depends on. Container names started before this one. */
  depends_on?: Array<string>;
  /** Health check. */
  healthcheck?: TestRuntime1ContainerHealthcheck | null;
  /** Restart policy. */
  restart?: "no" | "on-failure" | "unless-stopped" | "always";
  /** Ulimits. Soft limits, e.g. nofile; -1 = unlimited. */
  ulimits?: Record<string, number | string>;
}

/** object pool */
export interface TestRuntime1DriverPool {
  /** Max connections. Stroppy max_conns; unset preserves the driver default. */
  maxConns?: number | string | null;
  /** Min connections. Stroppy min_conns; unset preserves the driver default. */
  minConns?: number | string | null;
  /** Min idle connections. Stroppy min_idle_conns; unset preserves the driver default. */
  minIdleConns?: number | string | null;
  /** Max lifetime. Stroppy max_conn_lifetime; zero disables the lifetime limit. */
  maxConnLifetime?: string | null;
  /** Max idle time. Stroppy max_conn_idle_time; zero disables the lifetime limit. */
  maxConnIdleTime?: string | null;
  /** Description cache. Stroppy description_cache_capacity; unset preserves the driver default. */
  descriptionCacheCapacity?: number | string | null;
  /** Statement cache. Stroppy statement_cache_capacity; unset preserves the driver default. */
  statementCacheCapacity?: number | string | null;
  /** Driver log level. pgx tracer log level (traceLogLevel). */
  traceLogLevel?: "trace" | "debug" | "info" | "warn" | "error" | "none" | null;
  /** Query execution mode. pgx defaultQueryExecMode. */
  defaultQueryExecMode?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol" | null;
  /** Max open connections. Stroppy max_open_conns; unset preserves the driver default. */
  maxOpenConns?: number | string | null;
  /** Max idle connections. Stroppy max_idle_conns; unset preserves the driver default. */
  maxIdleConns?: number | string | null;
  /** Max lifetime. Stroppy conn_max_lifetime; zero disables the lifetime limit. */
  connMaxLifetime?: string | null;
  /** Max idle time. Stroppy conn_max_idle_time; zero disables the lifetime limit. */
  connMaxIdleTime?: string | null;
}

/** object postgres */
export interface TestRuntime1DriverPostgres {
  /** Max connections. Stroppy max_conns; unset preserves the driver default. */
  maxConns?: number | string | null;
  /** Min connections. Stroppy min_conns; unset preserves the driver default. */
  minConns?: number | string | null;
  /** Min idle connections. Stroppy min_idle_conns; unset preserves the driver default. */
  minIdleConns?: number | string | null;
  /** Max lifetime. Stroppy max_conn_lifetime; zero disables the lifetime limit. */
  maxConnLifetime?: string | null;
  /** Max idle time. Stroppy max_conn_idle_time; zero disables the lifetime limit. */
  maxConnIdleTime?: string | null;
  /** Description cache. Stroppy description_cache_capacity; unset preserves the driver default. */
  descriptionCacheCapacity?: number | string | null;
  /** Statement cache. Stroppy statement_cache_capacity; unset preserves the driver default. */
  statementCacheCapacity?: number | string | null;
  /** Driver log level. pgx tracer log level (traceLogLevel). */
  traceLogLevel?: "trace" | "debug" | "info" | "warn" | "error" | "none" | null;
  /** Query execution mode. pgx defaultQueryExecMode. */
  defaultQueryExecMode?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol" | null;
}

/** object sql */
export interface TestRuntime1DriverSql {
  /** Max open connections. Stroppy max_open_conns; unset preserves the driver default. */
  maxOpenConns?: number | string | null;
  /** Max idle connections. Stroppy max_idle_conns; unset preserves the driver default. */
  maxIdleConns?: number | string | null;
  /** Max lifetime. Stroppy conn_max_lifetime; zero disables the lifetime limit. */
  connMaxLifetime?: string | null;
  /** Max idle time. Stroppy conn_max_idle_time; zero disables the lifetime limit. */
  connMaxIdleTime?: string | null;
}

/** object insertProgress */
export interface TestRuntime1DriverInsertProgress {
  /** Enabled. Explicit insertProgress.enabled override; unset uses the mode. */
  enabled?: boolean | null;
  /** Mode. insertProgress.mode: where load progress goes. */
  mode?: "off" | "log" | "metrics" | "both";
  /** Interval. insertProgress.interval — progress cadence. */
  interval?: string;
  /** Stall after. insertProgress.stallAfter — warn when no rows moved for this long. */
  stallAfter?: string;
}

/** def driver */
export interface TestRuntime1Driver {
  /** Insert method. Fallback for load requests that leave their method unset (defaultInsertMethod); workloads normally choose themselves. */
  defaultInsertMethod?: "native" | "columnar" | "plain_bulk" | "plain_query" | null;
  /** Bulk size. Rows per bulk INSERT statement (bulkSize). [rows] */
  bulkSize?: number | string;
  /** Connection pool. Native Stroppy pool options; explicit driver settings take precedence over pool aliases. */
  pool?: TestRuntime1DriverPool;
  /** PostgreSQL driver. Native Stroppy postgres options; explicit driver settings take precedence over pool aliases. */
  postgres?: TestRuntime1DriverPostgres;
  /** SQL driver. Native Stroppy sql options; explicit driver settings take precedence over pool aliases. */
  sql?: TestRuntime1DriverSql;
  /** Load progress. insertProgress.* — load progress reporting. */
  insertProgress?: TestRuntime1DriverInsertProgress;
  /** Auth token. IAM token passed as authToken. */
  authToken?: string | null;
  /** User. Static credentials user (authUser). */
  authUser?: string | null;
  /** Password. Static credentials password (authPassword). */
  authPassword?: string | null;
  /** Skip TLS verification. tlsInsecureSkipVerify — testing only. */
  tlsInsecureSkipVerify?: boolean;
  /** CA file. Path inside the Stroppy container; use a shipped file or workload.ca_cert for inline PEM, not both. */
  caCertFile?: string;
}

/** object boot_disk */
export interface TestRuntime1MachineBootDisk {
  /** Physical block size. YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting. [bytes] */
  block_size?: number | string;
  /** Size. OS disk size in GiB; must fit the selected image. [GiB] */
  gb: number | string;
  /** Type. Cloud disk type. */
  type: string;
}

/** object  */
export interface TestRuntime1MachineItem {
  /** Physical block size. YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting. [bytes] */
  block_size?: number | string;
  /** Device name. */
  name: string;
  /** Size. [GB] */
  gb: number | string;
  /** Disk type. Provider disk type id, e.g. network-ssd (yandex) or gp3 (aws). */
  type: string;
  /** Filesystem. Automatic filesystem for mounted YC disks; omit for ext4. No mount means a raw block device. */
  filesystem?: "ext4" | "xfs";
  /** Mount options. mount/fstab options, e.g. noatime; omission uses defaults. */
  mount_options?: Array<string>;
  /** Mount point. Absolute path host_prep mounts the disk at; empty leaves it raw. */
  mount?: string;
}

/** def machine */
export interface TestRuntime1Machine {
  /** vCPU. */
  cpu?: number | string;
  /** Memory. [GB] */
  memory_gb?: number | string;
  /** Boot disk. Omit for a 40 GiB SSD OS disk. */
  boot_disk?: TestRuntime1MachineBootDisk | null;
  /** Guaranteed CPU. YC guaranteed CPU percentage; omission means 100. Unsupported on AWS. [%] */
  core_fraction?: number | string;
  /** Preemptible. Override the provider profile's preemptible/spot setting; explicit false is preserved. */
  preemptible?: boolean;
  /** Public IP. Override public addressing for this VM; outbound connectivity remains required for the agent and image pulls. */
  public_ip?: boolean;
  /** Extra disks. Secondary disks beyond the boot disk. */
  disks?: Array<TestRuntime1MachineItem>;
  /** Image. Resolved boot image (family id, image id or AMI id). */
  image?: string;
  /** Location. Zone (yandex) or availability zone (aws) the machine is created in. */
  location?: string;
  /** Instance type. Platform id (yandex) or EC2 instance type (aws) from the size table. */
  instance_type?: string;
  /** Labels. Provider labels; the run and role labels are added by the pipeline. */
  labels?: Record<string, string>;
}

/** variant baseline of workload */
export interface TestRuntime1SegmentWorkloadBaseline {
  script: "baseline";
  /** Load workers. Workers used to load each table (loadWorkers). */
  load_workers?: number | string;
  /** Rows. Rows loaded into the baseline probe table (rows). */
  rows?: number | string;
  /** Transaction isolation. Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none. */
  tx_isolation?: "read_uncommitted" | "read_committed" | "repeatable_read" | "serializable" | "db_default" | "conn" | "none" | null;
}

/** variant execute_sql of workload */
export interface TestRuntime1SegmentWorkloadExecuteSql {
  script: "execute_sql";
  /** Inline SQL. SQL text to execute (sqlBody); may start with a `--= name` marker to name the query. */
  sql_body?: string | null;
  /** SQL file. SQL file to execute (sqlFile): a file shipped in files. */
  sql_file?: string | null;
}

/** variant simple of workload */
export interface TestRuntime1SegmentWorkloadSimple {
  script: "simple";
}

/** variant tpcb/procs of workload */
export interface TestRuntime1SegmentWorkloadTpcbProcs {
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
export interface TestRuntime1SegmentWorkloadTpcbTx {
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
export interface TestRuntime1SegmentWorkloadTpccProcs {
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
export interface TestRuntime1SegmentWorkloadTpccTx {
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
export interface TestRuntime1SegmentWorkloadTpcds {
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
export interface TestRuntime1SegmentWorkloadTpchTx {
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
export interface TestRuntime1SegmentRun {
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
export interface TestRuntime1SegmentItem {
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
export interface TestRuntime1SegmentThresholds {
  /** p99 latency. Fail the segment when iteration_duration p99 exceeds this. [ms] */
  p99_ms?: number;
  /** Error rate. Fail the segment when failed_iterations / iterations exceeds this (0..1). [ratio] */
  error_rate?: number;
}

/** def segment */
export interface TestRuntime1Segment {
  /** Name. Segment id inside the workload; used as the phase label of the run and as a metric label. */
  name: string;
  /** Workload. Built-in stroppy workload and its typed parameters; `script` is the id passed to `stroppy run`. */
  workload: TestRuntime1SegmentWorkloadBaseline | TestRuntime1SegmentWorkloadExecuteSql | TestRuntime1SegmentWorkloadSimple | TestRuntime1SegmentWorkloadTpcbProcs | TestRuntime1SegmentWorkloadTpcbTx | TestRuntime1SegmentWorkloadTpccProcs | TestRuntime1SegmentWorkloadTpccTx | TestRuntime1SegmentWorkloadTpcds | TestRuntime1SegmentWorkloadTpchTx;
  /** Scenario. Load shape of the segment. */
  run: TestRuntime1SegmentRun;
  /** Only these steps. Run only the listed stroppy steps (--steps); empty = all steps. */
  steps?: Array<string>;
  /** Skip these steps. Skip the listed stroppy steps (--no-steps); stroppy rejects it together with steps. */
  no_steps?: Array<string>;
  /** Extra parameters. Typed stroppy flags this form does not model, by flag name without dashes (load-workers); values are parsed by stroppy. */
  extra_params?: Record<string, string>;
  /** Files. Extra files (SQL dialects, schemas, data) shipped with the segment. */
  files?: Array<TestRuntime1SegmentItem>;
  /** Thresholds. Pass/fail bounds the pipeline applies to the segment summary. */
  thresholds?: TestRuntime1SegmentThresholds;
  /** Random seed. Stroppy global.seed; zero uses Stroppy's random seed, a positive value makes generation reproducible. */
  seed?: number | string | null;
  /** Segment timeout. Execution deadline after the warmup wait, including container preparation and data load; unset uses duration plus headroom, or 24h for iterations. */
  timeout?: string | null;
  /** Warm-up. Idle wait before the segment starts, letting caches and replicas settle. */
  warmup?: string;
  /** Log level. Minimum stroppy log level (--log-level); debug traces parameter resolution. */
  log_level?: "debug" | "info" | "warn" | "error";
}

/** variant create of network */
export interface TestRuntime1YandexSettingsNetworkCreate {
  kind: "create";
  /** Subnet CIDR. IPv4 range of the subnet created for the run. */
  subnet_cidr?: string;
}

/** variant existing of network */
export interface TestRuntime1YandexSettingsNetworkExisting {
  kind: "existing";
  /** Network id. Existing VPC network the machines join. */
  network_id: string;
  /** Subnet id. Existing subnet in the chosen zone. */
  subnet_id: string;
  /** Security group id. Existing security group applied to every machine. */
  security_group_id?: string;
}

/** def yandex_settings */
export interface TestRuntime1YandexSettings {
  /** Cloud id. Yandex Cloud cloud id owning the folder. */
  cloud_id: string;
  /** Folder id. Folder every VM, disk and network of a run is created in. */
  folder_id: string;
  /** Zone. Availability zone of ru-central1 the run is placed in. */
  zone: "ru-central1-a" | "ru-central1-b" | "ru-central1-d" | "ru-central1-e";
  /** Distributed topology zones. Three physical zones for multi-zone topologies. When omitted, use zone and two other catalog zones. Single-zone topologies use zone. */
  zones?: Array<string>;
  /** Platform. Compute platform (CPU generation) the machines are created on. */
  platform_id?: "standard-v1" | "standard-v2" | "standard-v3" | "standard-v4a" | "amd-v1" | "highfreq-v3" | "highfreq-v4a";
  /** Network. Create a throwaway network for every run, or place runs into an existing one. */
  network?: TestRuntime1YandexSettingsNetworkCreate | TestRuntime1YandexSettingsNetworkExisting;
  /** Public IPs. Assign a one-to-one NAT public address to every machine. */
  public_ips?: boolean;
  /** Image family. Boot image family from the standard-images folder. */
  image_family?: "ubuntu-2404-lts" | "ubuntu-2204-lts" | "ubuntu-2004-lts" | "debian-12" | "centos-stream-9-oslogin";
  /** Preemptible. Use preemptible VMs: much cheaper, but stopped after 24h or under pressure — do not use for a measurement that must complete. */
  preemptible?: boolean;
}

/** object  */
export interface TestRuntime1Item {
  /** Role. Select containers by role, optionally narrowed by name. */
  role?: string;
  /** Name. Exact generated container name; at least one selector is required. */
  name?: string;
  /** Settings. Explicit fields replace defaults; env/ulimits merge by key and files merge by absolute path. Empty files list removes generated files. */
  set: TestRuntime1Container;
}

/** object  */
export interface TestRuntime1NetworkItem {
  /** Port. */
  port: number | string;
  /** Protocol. */
  proto?: "tcp" | "udp";
  /** Source CIDR. Who may reach the port. */
  cidr: string;
}

/** object network */
export interface TestRuntime1Network {
  /** CIDR. IPv4 range of the run network. */
  cidr?: string;
  /** Public IPs. Machines get a public address (needed when the agent has no private route out). */
  allow_public_ips?: boolean;
  /** Ingress. Security group openings beyond the intra-network traffic. */
  ingress?: Array<TestRuntime1NetworkItem>;
}

/** object  */
export interface TestRuntime1Item2Item {
  /** Container port. */
  container: number | string;
  /** Host port. Host-network port; remapping is not supported. */
  host: number | string;
}

/** object  */
export interface TestRuntime1Item2Item2 {
  /** Host path. */
  source: string;
  /** Container path. */
  target: string;
  /** Read-only. */
  ro?: boolean;
}

/** object  */
export interface TestRuntime1Item2Item3 {
  /** Path. Absolute path inside the container. */
  path: string;
  /** Content. Rendered config text (cfg.* schema Render output). */
  content: string;
  /** Mode. Octal file mode. */
  mode?: string;
}

/** object healthcheck */
export interface TestRuntime1Item2Healthcheck {
  /** Command. */
  cmd: Array<string>;
  /** Interval. */
  interval?: string;
  /** Retries. */
  retries?: number | string;
}

/** object  */
export interface TestRuntime1Item2 {
  /** Name. */
  name: string;
  /** Role. Role the container belongs to; used for logs, metrics and flows. */
  role: string;
  /** Machine. Name of the machine the container runs on. */
  machine: string;
  /** Image. Fully qualified docker image; every database is a container, no host packages. */
  image: string;
  /** Command. Argv replacing the image command. */
  cmd?: Array<string>;
  /** Entrypoint. Executable and arguments replacing the image entrypoint; omit to use the upstream image entrypoint. */
  entrypoint?: Array<string>;
  /** Environment. */
  env?: Record<string, string>;
  /** Ports. */
  ports?: Array<TestRuntime1Item2Item>;
  /** Mounts. */
  mounts?: Array<TestRuntime1Item2Item2>;
  /** Files. Configs rendered by the server and injected at start. */
  files?: Array<TestRuntime1Item2Item3>;
  /** Scrape path. Prometheus metrics path on the container, e.g. /metrics; empty = not scraped. */
  scrape?: string;
  /** Metrics port. HTTP port for the scrape path; when omitted, uses the first container port for compatibility. */
  scrape_port?: number | string;
  /** Depends on. Container names started before this one. */
  depends_on?: Array<string>;
  /** Health check. */
  healthcheck?: TestRuntime1Item2Healthcheck;
  /** Restart policy. */
  restart?: "no" | "on-failure" | "unless-stopped" | "always";
  /** Ulimits. Soft limits, e.g. nofile; -1 = unlimited. */
  ulimits?: Record<string, number | string>;
}

/** object  */
export interface TestRuntime1Item3 {
  /** Role. */
  role: string;
  /** Machine. Optional exact machine in this role; omission selects the whole role. */
  machine?: string;
  /** Kind. What the step does before any container starts. */
  kind: "sysctl" | "disks" | "script";
  /** Content. sysctl lines, mount spec or shell script, per kind. */
  content: string;
}

/** object  */
export interface TestRuntime1Item4 {
  /** Role. */
  role: string;
  /** URL. Metrics endpoint on the machine, e.g. http://127.0.0.1:9187/metrics. */
  url: string;
  /** Job. Name of the container to attach this scrape to; role must match its role. */
  job: string;
}

/** object  */
export interface TestRuntime1Item5 {
  /** From role. */
  from_role: string;
  /** To role. Target role; set this or external, not both. */
  to_role?: string;
  /** External target. Host outside the run (registry, OTLP collector); set this or to_role. */
  external?: string;
  /** Protocol. */
  protocol: "tcp" | "http" | "grpc" | "prometheus_pull" | "otlp";
  /** Port. */
  port: number | string;
  /** Label. Human label drawn on the topology view. */
  label?: string;
}

/** object workload */
export interface TestRuntime1Workload {
  /** Runner role. Role of the machine stroppy runs on. */
  runner_role?: string;
  /** Stroppy image. Resolved stroppy image from the catalog for the chosen version (6.0.0+). */
  stroppy_image?: string;
  /** Driver type. stroppy driverType of drivers.0. */
  driver_type?: "postgres" | "mysql" | "picodata" | "ydb" | "noop";
  /** Connection URL. drivers.0.url; may carry ${ip:...} placeholders the pipeline expands after provisioning. */
  url?: string;
  /** Driver options. Remaining drivers.0 keys of stroppy-config.json (bulkSize, pool, insertProgress, caCertFile, authToken…), already in stroppy's lowerCamel form. */
  driver?: TestRuntime1Driver;
  /** Segments. Ordered workload.segment@1 values; validated with the same schema as library workloads. */
  segments?: Array<TestRuntime1Segment>;
  /** Baseline. Baked workload.stroppy@1 baseline object; absent or disabled = no machine self-check. */
  baseline?: TestRuntime1Baseline;
  /** YDB credentials reference. Graphene secret containing YC service-account credentials; a short-lived IAM token is resolved on the runner into a temporary runtime config, never a retained artifact. */
  ydb_iam_credentials_secret?: string;
  /** CA certificate. PEM the pipeline writes next to the config and points caCertFile at. */
  ca_cert?: string;
}

/** object observability */
export interface TestRuntime1Observability {
  /** OTLP endpoint. Where agents forward stroppy metrics and logs. */
  otlp_endpoint?: string;
  /** OTLP headers. Comma-separated key=value headers stroppy sends with every export (otlpHeaders). */
  otlp_headers?: string;
  /** Labels. Labels stamped on every metric and log line of the run. */
  labels?: Record<string, string>;
}

/** root */
export interface TestRuntime1 {
  /** Machine overrides. Exact generated machine name to hardware overrides, applied after role presets. */
  machines?: Record<string, TestRuntime1Machine>;
  /** Container overrides. Applied in order; later matching entries win. Unknown selectors fail compilation. */
  containers?: Array<TestRuntime1Item>;
  /** Network. The network every machine of the run joins. */
  network?: TestRuntime1Network;
  /** Containers. Everything that runs on the machines: databases, proxies, exporters. */
  additional_containers?: Array<TestRuntime1Item2>;
  /** Host preparation. Pre-deploy steps, including automatic per-disk preparation, run on the machine itself. */
  host_prep?: Array<TestRuntime1Item3>;
  /** Scrapes. Agent-side Prometheus scrapes of exporters. */
  scrapes?: Array<TestRuntime1Item4>;
  /** Flows. Traffic relationships for the topology view; cloud security groups allow intra-network traffic and use network.ingress for extra openings. */
  flows?: Array<TestRuntime1Item5>;
  /** Workload. What stroppy runs and where. */
  workload?: TestRuntime1Workload;
  /** Observability. */
  observability?: TestRuntime1Observability;
  /** Expected metrics. Metric keys the run must report; a missing key degrades the result. */
  result_expectations?: Array<string>;
}
