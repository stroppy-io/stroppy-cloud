// GENERATED from schemapb schema cfg.orioledb.postgresql.conf@17 — do not edit.
// postgresql.conf for the OrioleDB build of PostgreSQL 17: the stock 17 parameters plus the orioledb.* settings.

/** root */
export interface CfgOrioledbPostgresqlConf17 {
  /** Shared buffers. Shared memory buffers for page caching. Upstream default 128MB; the server usually overrides it from the role's memory at RunSpec compile. [MB] */
  shared_buffers?: number | string;
  /** Huge pages. Request huge pages for the shared memory region. */
  huge_pages?: "try" | "on" | "off";
  /** Temp buffers. Per-session buffers for temporary tables. [MB] */
  temp_buffers?: number | string;
  /** Work mem. Memory per sort/hash operation before spilling to disk. Modeled in MB, so the sub-MB range of the upstream default is not reachable. [MB] */
  work_mem?: number | string;
  /** Hash mem multiplier. work_mem multiplier for hash-based plan nodes. */
  hash_mem_multiplier?: number;
  /** Maintenance work mem. Memory for VACUUM, CREATE INDEX and ALTER TABLE ADD FOREIGN KEY. [MB] */
  maintenance_work_mem?: number | string;
  /** Effective cache size. Planner-only estimate of the OS + PostgreSQL cache available to one query. Allocates nothing. [MB] */
  effective_cache_size?: number | string;
  /** WAL level. How much information is written to the WAL. `replica` is the minimum for streaming replication. */
  wal_level?: "minimal" | "replica" | "logical";
  /** Max WAL size. Soft limit on WAL growth between automatic checkpoints. [MB] */
  max_wal_size?: number | string;
  /** Min WAL size. WAL kept for recycling instead of being removed. [MB] */
  min_wal_size?: number | string;
  /** Checkpoint timeout. Maximum time between automatic WAL checkpoints. [s] */
  checkpoint_timeout?: number | string;
  /** Checkpoint completion target. Fraction of the checkpoint interval over which the checkpoint is spread. */
  checkpoint_completion_target?: number;
  /** WAL compression. Compress full-page images written to the WAL. lz4 and zstd require a build with --with-lz4 / --with-zstd (both available since PostgreSQL 15). */
  wal_compression?: "off" | "pglz" | "lz4" | "zstd";
  /** Synchronous commit. How much WAL durability a COMMIT waits for. */
  synchronous_commit?: "off" | "local" | "remote_write" | "on" | "remote_apply";
  /** WAL buffers. Shared memory for WAL not yet written to disk. `-1` derives it from shared_buffers; any other value must carry an explicit unit (a bare integer would mean 8kB blocks). */
  wal_buffers?: string;
  /** fsync. Force WAL and data file updates to physical storage. Turning it off makes crashes unrecoverable — benchmark use only. */
  fsync?: "on" | "off";
  /** Full page writes. Write the whole page to WAL on the first modification after a checkpoint; protects against torn pages. */
  full_page_writes?: "on" | "off";
  /** WAL writer delay. How often the WAL writer flushes WAL. [ms] */
  wal_writer_delay?: number | string;
  /** Max WAL senders. Maximum concurrent walsender processes; 0 disables replication. */
  max_wal_senders?: number | string;
  /** Max replication slots. Maximum number of replication slots the server can define. */
  max_replication_slots?: number | string;
  /** Hot standby. Allow read-only queries while the server is in recovery. */
  hot_standby?: "on" | "off";
  /** Hot standby feedback. Report the standby's oldest running xmin to the primary so vacuum does not remove rows it still needs. */
  hot_standby_feedback?: "on" | "off";
  /** WAL keep size. Minimum size of past WAL segments kept in pg_wal for standbys. [MB] */
  wal_keep_size?: number | string;
  /** Synchronous standby names. Standby list that synchronous_commit waits for, e.g. "ANY 1 (pg1, pg2)". Filled by the server from topology; when a Patroni cluster owns the config, Patroni manages this key instead. */
  synchronous_standby_names?: string | null;
  /** Sequential page cost. Planner cost of a sequentially fetched page. */
  seq_page_cost?: number;
  /** Random page cost. Planner cost of a non-sequentially fetched page; lower it for SSD/NVMe storage. */
  random_page_cost?: number;
  /** Effective I/O concurrency. Number of concurrent I/O requests the planner expects the storage to sustain. Upstream default 1 up to PostgreSQL 17. */
  effective_io_concurrency?: number | string;
  /** JIT. Allow JIT compilation of expressions; usually off for OLTP benchmarks, where it only adds latency. */
  jit?: "on" | "off";
  /** Default statistics target. Default number of ANALYZE histogram buckets / MCV entries per column. */
  default_statistics_target?: number | string;
  /** Autovacuum. Run the autovacuum launcher. */
  autovacuum?: "on" | "off";
  /** Autovacuum max workers. Maximum concurrent autovacuum worker processes. */
  autovacuum_max_workers?: number | string;
  /** Autovacuum naptime. Delay between autovacuum runs on any one database. [s] */
  autovacuum_naptime?: number | string;
  /** Vacuum threshold. Minimum number of updated/deleted tuples before a table is vacuumed. */
  autovacuum_vacuum_threshold?: number | string;
  /** Vacuum insert threshold. Minimum number of inserted tuples before a table is vacuumed; -1 disables insert-driven vacuums. */
  autovacuum_vacuum_insert_threshold?: number | string;
  /** Analyze threshold. Minimum number of changed tuples before a table is analyzed. */
  autovacuum_analyze_threshold?: number | string;
  /** Vacuum scale factor. Fraction of the table size added to autovacuum_vacuum_threshold. */
  autovacuum_vacuum_scale_factor?: number;
  /** Vacuum insert scale factor. Fraction of the table size added to autovacuum_vacuum_insert_threshold. */
  autovacuum_vacuum_insert_scale_factor?: number;
  /** Analyze scale factor. Fraction of the table size added to autovacuum_analyze_threshold. */
  autovacuum_analyze_scale_factor?: number;
  /** Autovacuum cost delay. Cost delay used by autovacuum workers; -1 reuses vacuum_cost_delay. [ms] */
  autovacuum_vacuum_cost_delay?: number;
  /** Autovacuum cost limit. Cost limit shared by all autovacuum workers; -1 reuses vacuum_cost_limit. */
  autovacuum_vacuum_cost_limit?: number | string;
  /** Vacuum cost delay. Sleep after the cost limit is exceeded during manual VACUUM; 0 disables cost-based delays. [ms] */
  vacuum_cost_delay?: number;
  /** Vacuum cost limit. Accumulated vacuum cost that triggers a sleep of vacuum_cost_delay. */
  vacuum_cost_limit?: number | string;
  /** Vacuum cost: page hit. Cost charged for a buffer found in shared_buffers. */
  vacuum_cost_page_hit?: number | string;
  /** Vacuum cost: page miss. Cost charged for a buffer that has to be read in. */
  vacuum_cost_page_miss?: number | string;
  /** Vacuum cost: page dirty. Cost charged when vacuum dirties a previously clean block. */
  vacuum_cost_page_dirty?: number | string;
  /** Listen addresses. TCP addresses to listen on. Product default is '*' rather than upstream 'localhost': every stroppy node is reached over the network by the load generator and the exporters. */
  listen_addresses?: string;
  /** Port. TCP port the server listens on. */
  port?: number | string;
  /** Max connections. Maximum concurrent client connections. */
  max_connections?: number | string;
  /** Superuser reserved connections. Connection slots out of max_connections reserved for superusers. */
  superuser_reserved_connections?: number | string;
  /** Reserved connections. Connection slots reserved for roles with pg_use_reserved_connections. Added in PostgreSQL 16. */
  reserved_connections?: number | string;
  /** SSL. Enable SSL/TLS connections; requires ssl_cert_file and ssl_key_file. */
  ssl?: "on" | "off";
  /** TCP keepalives idle. Idle time before the first TCP keepalive probe; 0 uses the OS default. [s] */
  tcp_keepalives_idle?: number | string;
  /** TCP keepalives interval. Interval between TCP keepalive retransmits; 0 uses the OS default. [s] */
  tcp_keepalives_interval?: number | string;
  /** TCP keepalives count. TCP keepalive probes lost before the connection is dropped; 0 uses the OS default. */
  tcp_keepalives_count?: number | string;
  /** Max worker processes. Maximum background worker processes the cluster may start. */
  max_worker_processes?: number | string;
  /** Max parallel workers. Maximum workers usable for parallel operations; bounded by max_worker_processes. */
  max_parallel_workers?: number | string;
  /** Max parallel workers per gather. Workers a single Gather node may start; 0 disables parallel query. */
  max_parallel_workers_per_gather?: number | string;
  /** Max parallel maintenance workers. Workers a single utility command (CREATE INDEX, VACUUM) may start. */
  max_parallel_maintenance_workers?: number | string;
  /** Max locks per transaction. Average number of object locks allocated per transaction slot. */
  max_locks_per_transaction?: number | string;
  /** Deadlock timeout. Time to wait on a lock before running deadlock detection. [ms] */
  deadlock_timeout?: number | string;
  /** Lock timeout. Abort a statement that waits longer than this for any lock; 0 disables. [ms] */
  lock_timeout?: number | string;
  /** Statement timeout. Abort any statement running longer than this; 0 disables. [ms] */
  statement_timeout?: number | string;
  /** Idle in transaction timeout. Terminate a session idle inside an open transaction for longer than this; 0 disables. [ms] */
  idle_in_transaction_session_timeout?: number | string;
  /** Transaction timeout. Terminate a session whose transaction runs longer than this; 0 disables. Added in PostgreSQL 17. [ms] */
  transaction_timeout?: number | string;
  /** Logging collector. Capture stderr into rotated log files. Off in stroppy: journald/vector collect stderr. */
  logging_collector?: "on" | "off";
  /** Log destination. Comma-separated log sinks: stderr, csvlog, jsonlog, syslog. */
  log_destination?: string;
  /** Log min duration. Log statements running at least this long; 0 logs all, -1 disables. [ms] */
  log_min_duration_statement?: number | string;
  /** Log checkpoints. Log each checkpoint and restartpoint. */
  log_checkpoints?: "on" | "off";
  /** Log connections. Log every attempted connection. A boolean up to PostgreSQL 17; an aspect list from 18. */
  log_connections?: "on" | "off";
  /** Log disconnections. Log session end and duration. */
  log_disconnections?: "on" | "off";
  /** Log lock waits. Log a session that waits longer than deadlock_timeout for a lock. */
  log_lock_waits?: "on" | "off";
  /** Log temp files. Log temporary files at least this large; 0 logs all, -1 disables. [kB] */
  log_temp_files?: number | string;
  /** Log autovacuum min duration. Log autovacuum actions taking at least this long; 0 logs all, -1 disables. [ms] */
  log_autovacuum_min_duration?: number | string;
  /** Log line prefix. printf-style prefix on every stderr log line. */
  log_line_prefix?: string;
  /** Log statement. Which statement kinds to log in full. */
  log_statement?: "none" | "ddl" | "mod" | "all";
  /** Preloaded libraries. Shared libraries preloaded at server start; rendered into shared_preload_libraries. Filled by the server from the recipe (pg_stat_statements for the metrics dashboards, orioledb for the OrioleDB engine). */
  extensions?: Array<string> | null;
  /** shared_preload_libraries. The `extensions` list joined with commas — the literal shared_preload_libraries value. */
  readonly shared_preload_libraries?: string;
  /** pg_stat_statements.max. Number of statements tracked by pg_stat_statements. Rendered as `pg_stat_statements.max` (schemapb field names must be identifiers, so the dot cannot be in the field name). */
  pg_stat_statements_max?: number | string;
  /** pg_stat_statements.track. Which statements pg_stat_statements counts. Rendered as `pg_stat_statements.track`. */
  pg_stat_statements_track?: "none" | "top" | "all";
  /** Rendered pg_stat_statements settings. The pg_stat_statements.* lines, emitted only when the extension is actually preloaded. */
  readonly pg_stat_statements_rendered?: string;
  /** Timezone. Server time zone. Product default UTC so every run's timestamps line up with the metrics backend. */
  timezone?: string;
  /** Message locale. Locale for server log messages. Product default C so log parsers see English, untranslated messages. */
  lc_messages?: string;
  /** Default text search config. Text search configuration used when none is given explicitly. */
  default_text_search_config?: string;
  /** orioledb.main_buffers. Shared memory OrioleDB uses to cache hot pages; the OrioleDB counterpart of shared_buffers. [MB] */
  orioledb_main_buffers?: number | string;
  /** orioledb.undo_buffers. Ring buffer holding older row and page versions. Leave unset for the engine default, which grows with the maximum process count; an explicit value must meet the engine's process-dependent minimum. [MB] */
  orioledb_undo_buffers?: number | string | null;
  /** orioledb.free_tree_buffers. Shared memory for the free-space metadata of compressed tables. [MB] */
  orioledb_free_tree_buffers?: number | string;
  /** orioledb.catalog_buffers. Shared memory for table metadata. [MB] */
  orioledb_catalog_buffers?: number | string;
  /** orioledb.temp_buffers. Per-backend buffer pool for OrioleDB temporary tables. [MB] */
  orioledb_temp_buffers?: number | string;
  /** orioledb.xid_buffers. In-memory buffer for OrioleDB transaction ids. [MB] */
  orioledb_xid_buffers?: number | string;
  /** orioledb.checkpoint_completion_ratio. Share of the checkpoint window spent on OrioleDB tables; the remainder goes to heap tables. */
  orioledb_checkpoint_completion_ratio?: number;
  /** orioledb.max_io_concurrency. Cap on concurrent OrioleDB I/O operations; 0 lets OrioleDB derive it from main_buffers. */
  orioledb_max_io_concurrency?: number | string;
  /** orioledb.bgwriter_num_workers. Background writer processes flushing OrioleDB pages. */
  orioledb_bgwriter_num_workers?: number | string;
  /** orioledb.recovery_pool_size. Worker processes replaying OrioleDB WAL during recovery. */
  orioledb_recovery_pool_size?: number | string;
  /** orioledb.recovery_idx_pool_size. Workers building indexes in parallel during recovery. */
  orioledb_recovery_idx_pool_size?: number | string;
  /** orioledb.recovery_queue_size. Shared memory for the message queues between recovery workers. [MB] */
  orioledb_recovery_queue_size?: number | string;
  /** orioledb.default_compress. Default compression level for new OrioleDB tables; -1 disables compression. */
  orioledb_default_compress?: number | string;
  /** orioledb.serializable. How OrioleDB answers a request for SERIALIZABLE isolation: table_lock takes a coarse ExclusiveLock per relation, error rejects the transaction, repeatable_read silently downgrades it. */
  orioledb_serializable?: "table_lock" | "error" | "repeatable_read";
  /** orioledb.use_mmap. Access the raw block device through mmap instead of read/write; only with device_filename. */
  orioledb_use_mmap?: "on" | "off";
  /** orioledb.device_length. Usable length of the raw block device; only meaningful with device_filename, and 0 (the upstream default) means no device. [MB] */
  orioledb_device_length?: number | string;
  /** orioledb.enable_rewind. Experimental undo-based rewind: keeps committed transactions rewindable. */
  orioledb_enable_rewind?: "on" | "off";
  /** orioledb.enable_stopevents. Debug-only stop events used by the OrioleDB test suite; costs throughput. */
  orioledb_enable_stopevents?: "on" | "off";
  /** orioledb.debug_disable_bgwriter. Debug-only: never start the OrioleDB background writer. */
  orioledb_debug_disable_bgwriter?: "on" | "off";
  /** orioledb.device_filename. Raw block device OrioleDB stores data on instead of the filesystem. Unset means normal files. */
  orioledb_device_filename?: string | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Rendered extra settings. The `custom` map joined into config lines; the template prints it at the end of the file. */
  readonly custom_rendered?: string;
}
