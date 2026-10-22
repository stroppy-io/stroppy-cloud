// GENERATED from schemapb schema db.orioledb.params@1 — do not edit.
// OrioleDB (containerised PostgreSQL storage engine) topology and options.

/** root */
export interface DbOrioledbParams1 {
  /** Tuning. auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto. */
  tuning?: "auto" | "stock";
  /** Image tag. Tag of the orioledb/orioledb image; the pgNN suffix picks the PostgreSQL major the engine is patched into. */
  image_tag: "latest-pg18" | "latest-pg17" | "latest-pg16" | "beta19-pg18" | "beta19-pg17" | "beta19-pg16" | "beta18-pg18" | "beta18-pg17" | "beta18-pg16" | "beta17-pg18" | "beta17-pg17" | "beta17-pg16";
  /** Ubuntu base image. Use the -ubuntu flavor of the tag instead of the default Alpine one; needed when the workload wants glibc locales. */
  ubuntu_base?: boolean;
  /** Streaming replicas. Physical async standbys; OrioleDB has no supported failover manager, so these never get promoted automatically. */
  replicas?: number | string;
  /** HAProxy nodes. HAProxy instances in front of the master (and read-only backends for the replicas). */
  haproxy?: number | string;
  /** Main buffers. orioledb.main_buffers of every database server, the page cache of OrioleDB tables. Unset = auto tuning sizes it from the machine (30% of RAM); stock tuning keeps the 64MB upstream default. A main_buffers set in the config wins. [MB] */
  main_buffers_mb?: number | string | null;
  /** Shared buffers. Retired: replaced by main_buffers_mb; a stored value is ignored. [MB] */
  shared_buffers_mb?: number | string;
  /** initdb locale. Locale passed to initdb inside the container; C.UTF-8 keeps collation cheap and image-independent. */
  initdb_locale?: string;
  /** OrioleDB by default. Set default_table_access_method = orioledb so unqualified CREATE TABLE lands on the OrioleDB engine rather than heap. */
  default_table_access_method?: boolean;
  /** Init SQL. SQL executed on the master once the container is healthy, before the workload. */
  init_sql?: string;
}
