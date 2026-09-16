// GENERATED from schemapb schema cfg.exporter.postgres@1 — do not edit.
// postgres_exporter flags and environment (prometheus-community/postgres_exporter v0.20.x).

/** root */
export interface CfgExporterPostgres1 {
  /** DATA_SOURCE_NAME. libpq connection string or URI the exporter scrapes, e.g. postgresql://exporter@10.0.0.11:5432/postgres?sslmode=disable. Filled by the server from the topology; it carries a password, so it is a secret. */
  data_source_name?: string | null;
  /** Listen port. Port the exporter serves /metrics on; rendered as --web.listen-address=:<port>. */
  web_listen_port?: number | string;
  /** Telemetry path. HTTP path the metrics are exposed under. */
  web_telemetry_path?: string;
  /** Log level. Exporter log verbosity. */
  log_level?: "debug" | "info" | "warn" | "error";
  /** Log format. Exporter log encoding. */
  log_format?: "logfmt" | "json";
  /** Disable default metrics. Drop every built-in metric and export only the custom queries. */
  disable_default_metrics?: "on" | "off";
  /** Metric prefix. Prefix of every exported metric name. */
  metric_prefix?: string;
  /** Collection timeout. Timeout for one scrape, as a Go duration; a slow database is reported as a failed scrape instead of stalling Prometheus. */
  collection_timeout?: string;
  /** Auto-discover databases. Scrape every database on the server, not just the one in DATA_SOURCE_NAME. Deprecated upstream. */
  auto_discover_databases?: "on" | "off";
  /** Exclude databases. Comma-separated databases to skip when auto-discovery is on. Deprecated upstream. */
  exclude_databases?: string;
  /** Include databases. Comma-separated databases to restrict auto-discovery to. Deprecated upstream. */
  include_databases?: string;
  /** collector.database. Per-database size and connection counts. Upstream default: on. */
  collector_database?: "on" | "off";
  /** collector.database_wraparound. Transaction-id and multixact age per database. Upstream default: off. */
  collector_database_wraparound?: "on" | "off";
  /** collector.locks. Lock counts per database and lock mode. Upstream default: on. */
  collector_locks?: "on" | "off";
  /** collector.long_running_transactions. Age and count of long-running transactions. Upstream default: off. */
  collector_long_running_transactions?: "on" | "off";
  /** collector.postmaster. Postmaster start time. Upstream default: off. */
  collector_postmaster?: "on" | "off";
  /** collector.process_idle. Histogram of idle backend times per application. Upstream default: off. */
  collector_process_idle?: "on" | "off";
  /** collector.replication. Replication lag and is-replica flag. Upstream default: on. */
  collector_replication?: "on" | "off";
  /** collector.replication_slots. Per-slot WAL retention and activity. Upstream default: on. */
  collector_replication_slots?: "on" | "off";
  /** collector.roles. Per-role connection limits. Upstream default: on. */
  collector_roles?: "on" | "off";
  /** collector.settings. The server's pg_settings as metrics. Upstream default: on. */
  collector_settings?: "on" | "off";
  /** collector.stat_activity. Backend counts by state, from pg_stat_activity. Upstream default: on. */
  collector_stat_activity?: "on" | "off";
  /** collector.stat_activity_autovacuum. Age of running autovacuum workers. Upstream default: off. */
  collector_stat_activity_autovacuum?: "on" | "off";
  /** collector.stat_archiver. WAL archiver successes and failures. Upstream default: on. */
  collector_stat_archiver?: "on" | "off";
  /** collector.stat_bgwriter. Background writer and checkpoint counters. Upstream default: on. */
  collector_stat_bgwriter?: "on" | "off";
  /** collector.stat_checkpointer. pg_stat_checkpointer, the PostgreSQL 17+ split of the bgwriter view. Upstream default: off. */
  collector_stat_checkpointer?: "on" | "off";
  /** collector.stat_database. pg_stat_database: transactions, tuples, conflicts, deadlocks. Upstream default: on. */
  collector_stat_database?: "on" | "off";
  /** collector.stat_progress_vacuum. Progress of running VACUUMs. Upstream default: on. */
  collector_stat_progress_vacuum?: "on" | "off";
  /** collector.stat_replication. Per-walsender byte positions and lag. Upstream default: on. */
  collector_stat_replication?: "on" | "off";
  /** collector.stat_statements. Per-statement time and call counts. Upstream default off; on in stroppy because the query dashboards read it (it needs pg_stat_statements preloaded). Upstream default: off. */
  collector_stat_statements?: "on" | "off";
  /** collector.stat_user_tables. Per-table scan, tuple and vacuum counters. Upstream default: on. */
  collector_stat_user_tables?: "on" | "off";
  /** collector.stat_wal_receiver. Standby-side WAL receiver state. Upstream default: off. */
  collector_stat_wal_receiver?: "on" | "off";
  /** collector.statio_user_indexes. Per-index block read/hit counters. Upstream default: off. */
  collector_statio_user_indexes?: "on" | "off";
  /** collector.statio_user_tables. Per-table block read/hit counters. Upstream default: on. */
  collector_statio_user_tables?: "on" | "off";
  /** collector.wal. WAL segment count and size. Upstream default: on. */
  collector_wal?: "on" | "off";
  /** collector.xlog_location. Current WAL LSN as a number. Upstream default: off. */
  collector_xlog_location?: "on" | "off";
  /** collector.buffercache_summary. pg_buffercache_summary; needs the pg_buffercache extension. Upstream default: off. */
  collector_buffercache_summary?: "on" | "off";
  /** Collector flags. Every collector rendered as --collector.<name> or --no-collector.<name>, in table order. */
  readonly collector_flags?: string;
  /** Command line. The full postgres_exporter argument list. */
  readonly opts?: string;
}
