// GENERATED from schemapb schema cfg.my.cnf@8 — do not edit.
// MySQL 8.0 server configuration rendered to /etc/mysql/my.cnf ([mysqld] section).

/** root */
export interface CfgMyCnf8 {
  /** Buffer pool size. InnoDB buffer pool; the single most important memory setting. Upstream default 128M. [MB] */
  innodb_buffer_pool_size?: number | string;
  /** Buffer pool instances. Number of buffer pool regions; only meaningful when the pool is larger than 1G. */
  innodb_buffer_pool_instances?: number | string;
  /** Redo log capacity. Total redo log size. Replaces the deprecated innodb_log_file_size/innodb_log_files_in_group. [MB] */
  innodb_redo_log_capacity?: number | string;
  /** Flush log at commit. 1 = ACID (flush+fsync on commit), 2 = flush on commit and fsync once a second, 0 = neither. */
  innodb_flush_log_at_trx_commit?: number | string;
  /** Flush method. Data/log flush method. Upstream default is fsync; stroppy defaults to O_DIRECT so the OS page cache does not double-buffer the pool during a benchmark. */
  innodb_flush_method?: "fsync" | "O_DIRECT" | "O_DIRECT_NO_FSYNC" | "O_DSYNC";
  /** I/O capacity. Background page-flush budget in IOPS. Upstream default 200 for 8.0. [IOPS] */
  innodb_io_capacity?: number | string;
  /** I/O capacity max. Upper flush budget when InnoDB has to catch up; must be >= innodb_io_capacity. [IOPS] */
  innodb_io_capacity_max?: number | string;
  /** Read I/O threads. Background read threads. */
  innodb_read_io_threads?: number | string;
  /** Write I/O threads. Background write threads. */
  innodb_write_io_threads?: number | string;
  /** Thread concurrency. Cap on threads inside InnoDB; 0 = unlimited (the default). */
  innodb_thread_concurrency?: number | string;
  /** Lock wait timeout. How long a transaction waits for a row lock before it is rolled back. [s] */
  innodb_lock_wait_timeout?: number | string;
  /** Doublewrite buffer. Torn-page protection. OFF trades crash safety for write throughput. */
  innodb_doublewrite?: "ON" | "OFF";
  /** File per table. Store each table in its own .ibd tablespace. */
  innodb_file_per_table?: "ON" | "OFF";
  /** Dedicated server. Let InnoDB size the buffer pool and redo log from the machine's RAM, ignoring the values above. */
  innodb_dedicated_server?: "ON" | "OFF";
  /** Binary log base name. Base name of the binary log files; binary logging is on by default since 8.0. */
  log_bin?: string;
  /** Binlog format. Row-based logging is the default and the only format group replication accepts. */
  binlog_format?: "ROW" | "STATEMENT" | "MIXED";
  /** Binlog row image. How much of each row goes into the binary log. */
  binlog_row_image?: "full" | "minimal" | "noblob";
  /** Sync binlog. fsync the binary log every N commit groups; 1 is the durable default, 0 leaves it to the OS. */
  sync_binlog?: number | string;
  /** Binlog retention. Binary log purge age; 0 disables automatic purging. Upstream default 30 days. [s] */
  binlog_expire_logs_seconds?: number | string;
  /** GTID mode. GTID-based replication. Group replication requires ON. */
  gtid_mode?: "OFF" | "OFF_PERMISSIVE" | "ON_PERMISSIVE" | "ON";
  /** Enforce GTID consistency. Reject statements that cannot be logged transactionally; ON is required with gtid_mode=ON. */
  enforce_gtid_consistency?: "OFF" | "WARN" | "ON";
  /** Log replica updates. Write replicated changes to this server's own binary log — required for chained replication and for group replication. */
  log_replica_updates?: "ON" | "OFF";
  /** Semisync source. Semisynchronous source side; loads semisync_source.so via plugin_load_add. */
  rpl_semi_sync_source_enabled?: "ON" | "OFF";
  /** Semisync timeout. How long the source waits for a replica ack before falling back to async. [ms] */
  rpl_semi_sync_source_timeout?: number | string;
  /** Semisync replica. Semisynchronous replica side; loads semisync_replica.so via plugin_load_add. */
  rpl_semi_sync_replica_enabled?: "ON" | "OFF";
  /** Applier workers. Parallel applier threads on the replica; 0 = single-threaded. */
  replica_parallel_workers?: number | string;
  /** Preserve commit order. Commit in source order when applying in parallel. */
  replica_preserve_commit_order?: "ON" | "OFF";
  /** Read only. Reject writes from clients without CONNECTION_ADMIN — set on replicas. */
  read_only?: "ON" | "OFF";
  /** Super read only. Reject writes from every account, administrators included; implies read_only. */
  super_read_only?: "ON" | "OFF";
  /** Group replication. Enable the group replication plugin; renders the whole group_replication_* block. */
  group_replication?: boolean;
  /** Group name. UUID naming the group; identical on every member. */
  group_replication_group_name?: string | null;
  /** Start on boot. Join the group automatically at server start. */
  group_replication_start_on_boot?: "ON" | "OFF";
  /** Bootstrap group. Create the group instead of joining it — exactly one member, exactly once. */
  group_replication_bootstrap_group?: "ON" | "OFF";
  /** Single primary mode. One writable primary (ON) or multi-primary (OFF). */
  group_replication_single_primary_mode?: "ON" | "OFF";
  /** Update-everywhere checks. Multi-primary safety checks; must be OFF in single-primary mode. */
  group_replication_enforce_update_everywhere_checks?: "ON" | "OFF";
  /** Transaction consistency. Group synchronization before or after transactions. BEFORE waits for preceding transactions before reads, allowing read-after-write through replicas; this is separate from SQL transaction isolation. */
  group_replication_consistency?: "EVENTUAL" | "BEFORE_ON_PRIMARY_FAILOVER" | "BEFORE" | "AFTER" | "BEFORE_AND_AFTER" | null;
  /** Bind address. Addresses mysqld listens on; * or 0.0.0.0 for all IPv4. */
  bind_address?: string;
  /** Port. TCP port for the classic MySQL protocol. */
  port?: number | string;
  /** Max connections. Maximum simultaneous client connections. */
  max_connections?: number | string;
  /** Max connect errors. Interrupted handshakes from one host before it is blocked. */
  max_connect_errors?: number | string;
  /** Thread cache size. Threads kept for reuse after a client disconnects. */
  thread_cache_size?: number | string;
  /** Wait timeout. Idle seconds before a non-interactive connection is closed. [s] */
  wait_timeout?: number | string;
  /** Interactive timeout. Idle seconds before an interactive connection is closed. [s] */
  interactive_timeout?: number | string;
  /** Max allowed packet. Largest single packet or generated string. [MB] */
  max_allowed_packet?: number | string;
  /** Table open cache. Open table handles cached across all sessions. */
  table_open_cache?: number | string;
  /** Table definition cache. Cached table definitions. */
  table_definition_cache?: number | string;
  /** Temp table size. Largest in-memory internal temporary table before it spills to disk. [MB] */
  tmp_table_size?: number | string;
  /** Max heap table size. Largest user-created MEMORY table; caps tmp_table_size too. [MB] */
  max_heap_table_size?: number | string;
  /** Open files limit. File descriptors mysqld asks the OS for. */
  open_files_limit?: number | string;
  /** SQL mode. Comma-separated SQL modes; the value below is the 8.0/8.4 upstream default. */
  sql_mode?: string;
  /** Server character set. Default character set of the server. */
  character_set_server?: "utf8mb4" | "utf8mb3" | "latin1";
  /** Server collation. Default collation; must belong to character_set_server. */
  collation_server?: string;
  /** Default time zone. Server time zone; a fixed offset keeps benchmark timestamps reproducible. */
  default_time_zone?: string;
  /** Lower case table names. 0 = case-sensitive names as stored, 1 = lowercased, 2 = stored as given, compared lowercase. Fixed at initialization. */
  lower_case_table_names?: number | string;
  /** Transaction isolation. Default isolation level for new sessions. */
  transaction_isolation?: "READ-UNCOMMITTED" | "READ-COMMITTED" | "REPEATABLE-READ" | "SERIALIZABLE";
  /** Performance schema. Instrumentation engine; needed by mysqld_exporter's perf_schema collectors. */
  performance_schema?: "ON" | "OFF";
  /** Max digest length. Bytes kept per normalized statement digest. [B] */
  performance_schema_max_digest_length?: number | string;
  /** General log. Log every statement — very expensive, off during load tests. */
  general_log?: "ON" | "OFF";
  /** Slow query log. Log statements slower than long_query_time. */
  slow_query_log?: "ON" | "OFF";
  /** Long query time. Slow-query threshold; fractional seconds are allowed. [s] */
  long_query_time?: number;
  /** Slow log extra fields. Add per-statement counters to slow-log records. */
  log_slow_extra?: "ON" | "OFF";
  /** Error log verbosity. 1 = errors, 2 = errors and warnings, 3 = adds notes. */
  log_error_verbosity?: number | string;
  /** Applier parallel type. How the applier partitions work. Removed in 8.4, where LOGICAL_CLOCK is always used. */
  replica_parallel_type?: "LOGICAL_CLOCK" | "DATABASE";
  /** Write set extraction. Write-set hashing algorithm; XXHASH64 is required by group replication. Removed in 8.4, where write sets are always extracted. */
  transaction_write_set_extraction?: "XXHASH64" | "MURMUR32" | "OFF";
  /** Dependency tracking. Source of the parallelization info written into the binary log. Removed in 8.4, where WRITESET is always used. */
  binlog_transaction_dependency_tracking?: "COMMIT_ORDER" | "WRITESET" | "WRITESET_SESSION";
  /** Default auth plugin. Authentication plugin for new accounts. Removed in 8.4 — use authentication_policy there. */
  default_authentication_plugin?: "caching_sha2_password" | "mysql_native_password" | "sha256_password";
  /** Server id. Unique replication id; filled by the server from topology. */
  server_id?: number | string | null;
  /** Report host. Address this member reports to the source/group; filled by the server from topology. */
  report_host?: string | null;
  /** GR local address. host:port of this member's group communication endpoint; filled by the server from topology. */
  group_replication_local_address?: string | null;
  /** GR group seeds. Comma-separated seed endpoints of the group; filled by the server from topology. */
  group_replication_group_seeds?: string | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Cluster lines. Rendered topology lines (server_id, report_host). */
  readonly cluster_lines?: string;
  /** Semisync lines. plugin_load_add and rpl_semi_sync_* lines, emitted only when semisync is on. */
  readonly semisync_lines?: string;
  /** Group replication lines. The whole group_replication_* block, emitted only when group replication is on. */
  readonly group_replication_lines?: string;
  /** Rendered extra settings. The `custom` map joined into config lines; the template prints it at the end of the file. */
  readonly custom_rendered?: string;
}
