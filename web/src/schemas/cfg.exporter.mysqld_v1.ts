// GENERATED from schemapb schema cfg.exporter.mysqld@1 — do not edit.
// prometheus/mysqld_exporter 0.15/0.16 command line, rendered as one flag per line.

/** root */
export interface CfgExporterMysqld1 {
  /** MySQL address. host:port of the mysqld to scrape; filled by the server from topology. */
  mysqld_address?: string | null;
  /** MySQL user. Account used for scraping; needs PROCESS, REPLICATION CLIENT and SELECT. */
  mysqld_username?: string;
  /** MySQL password. Password for that account; filled by the server. Never rendered into the flags file — it is passed as MYSQLD_EXPORTER_PASSWORD or written into config.my-cnf. */
  mysqld_password?: string | null;
  /** Credentials file. my.cnf whose [client] section holds the credentials; its options override the flags. */
  config_my_cnf?: string;
  /** Listen port. Port of the /metrics endpoint (--web.listen-address). */
  listen_port?: number | string;
  /** Log level. Exporter log verbosity. */
  log_level?: "debug" | "info" | "warn" | "error";
  /** Max open connections. Size of the exporter's connection pool to mysqld. */
  max_open_connections?: number | string;
  /** Query timeout. Per-scraper query timeout; 0 disables it. [s] */
  query_timeout?: number | string;
  /** collect.global_status. SHOW GLOBAL STATUS — the core counter set. Upstream default ON. */
  global_status?: "ON" | "OFF";
  /** collect.global_variables. SHOW GLOBAL VARIABLES — server configuration as metrics. Upstream default ON. */
  global_variables?: "ON" | "OFF";
  /** collect.slave_status. SHOW REPLICA/SLAVE STATUS — replication lag and errors. Upstream default ON. */
  slave_status?: "ON" | "OFF";
  /** collect.info_schema.innodb_metrics. information_schema.innodb_metrics — the detailed InnoDB counters. Upstream default OFF. */
  info_schema_innodb_metrics?: "ON" | "OFF";
  /** collect.info_schema.innodb_cmp. information_schema.innodb_cmp — compression statistics. Upstream default ON. */
  info_schema_innodb_cmp?: "ON" | "OFF";
  /** collect.info_schema.innodb_cmpmem. information_schema.innodb_cmpmem — compressed buffer pool statistics. Upstream default ON. */
  info_schema_innodb_cmpmem?: "ON" | "OFF";
  /** collect.info_schema.processlist. information_schema.processlist — per-state connection counts; costly on busy servers. Upstream default OFF. */
  info_schema_processlist?: "ON" | "OFF";
  /** collect.info_schema.tables. information_schema.tables — per-table size and row counts. Upstream default ON. */
  info_schema_tables?: "ON" | "OFF";
  /** collect.info_schema.query_response_time. Query response time histogram (Percona/MariaDB only). Upstream default ON. */
  info_schema_query_response_time?: "ON" | "OFF";
  /** collect.perf_schema.eventswaits. performance_schema wait events summary. Upstream default OFF. */
  perf_schema_eventswaits?: "ON" | "OFF";
  /** collect.perf_schema.eventsstatements. performance_schema statement digests — the per-query latency source. Upstream default OFF. */
  perf_schema_eventsstatements?: "ON" | "OFF";
  /** collect.perf_schema.tableiowaits. performance_schema table I/O waits. Upstream default OFF. */
  perf_schema_tableiowaits?: "ON" | "OFF";
  /** collect.perf_schema.indexiowaits. performance_schema index I/O waits. Upstream default OFF. */
  perf_schema_indexiowaits?: "ON" | "OFF";
  /** collect.perf_schema.file_events. performance_schema file I/O events. Upstream default OFF. */
  perf_schema_file_events?: "ON" | "OFF";
  /** collect.binlog_size. Total size of the binary logs on disk. Upstream default OFF. */
  binlog_size?: "ON" | "OFF";
  /** collect.engine_innodb_status. SHOW ENGINE INNODB STATUS — parsed into metrics. Upstream default OFF. */
  engine_innodb_status?: "ON" | "OFF";
  /** collect.auto_increment.columns. Auto-increment column headroom per table. Upstream default OFF. */
  auto_increment_columns?: "ON" | "OFF";
  /** collect.heartbeat. pt-heartbeat table based replication delay. Upstream default OFF. */
  heartbeat?: "ON" | "OFF";
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Collector flags. The collect.* toggles rendered as --collect.x / --no-collect.x, one per line. */
  readonly collector_flags?: string;
  /** Rendered extra flags. The `custom` map joined into --key=value flags. */
  readonly custom_rendered?: string;
  /** Target flags. --mysqld.address, emitted only once topology filled it in. */
  readonly target_flags?: string;
}
