// GENERATED from schemapb schema db.postgres.params@1 — do not edit.
// PostgreSQL topology and options: version, replicas, HA machinery, extensions.

/** root */
export interface DbPostgresParams1 {
  /** PostgreSQL version. Server major version; packages come from the PGDG apt repository (apt.postgresql.org). */
  version: "18" | "17" | "16" | "15";
  /** Streaming replicas. Physical streaming standbys next to the primary; 0 = a single node. */
  replicas?: number | string;
  /** Synchronous replicas. How many of the replicas commit synchronously (synchronous_standby_names); must not exceed replicas. */
  sync_replicas?: number | string;
  /** High availability. none = plain primary/standby without failover; patroni = Patroni owns postgres and elects the leader through etcd. */
  ha: "none" | "patroni";
  /** etcd nodes. Size of the etcd DCS backing Patroni; odd numbers only — 1 for a toy stand, 3 to survive one loss. */
  etcd_nodes?: 1 | 3;
  /** HAProxy nodes. HAProxy instances routing clients to the current primary (httpchk /primary against the Patroni REST API). */
  haproxy?: number | string;
  /** PgBouncer. Collocate a PgBouncer pooler with every database node. */
  pgbouncer?: boolean;
  /** WAL archiving. Turn on archive_mode and archive WAL segments to local storage on the primary. */
  wal_archive?: boolean;
  /** Extensions. CREATE EXTENSION on the benchmark database after initdb; preload-only ones are added to shared_preload_libraries too. */
  extensions?: Array<"pg_stat_statements" | "pg_buffercache" | "pg_prewarm" | "pg_trgm" | "vector" | "pg_partman" | "postgis" | "hypopg" | "pg_cron">;
  /** initdb locale. Locale passed to initdb --locale; C.UTF-8 keeps collation cheap and stable across images. */
  locale?: string;
  /** Init SQL. SQL executed on the primary once the cluster is up, before the workload (schema tweaks, roles, GUC overrides). */
  init_sql?: string;
}
