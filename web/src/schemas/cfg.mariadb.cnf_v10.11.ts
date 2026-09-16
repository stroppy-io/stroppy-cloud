// GENERATED from schemapb schema cfg.mariadb.cnf@10.11 — do not edit.
// MariaDB 10.11 server configuration rendered to /etc/mysql/mariadb.conf.d/50-server.cnf ([mariadb] section).

/** root */
export interface CfgMariadbCnf10_11 {
  /** Buffer pool size. InnoDB buffer pool. Upstream default 128M. [MB] */
  innodb_buffer_pool_size?: number | string;
  /** Redo log size. Size of the redo log. Unlike MySQL 8.0.30+, MariaDB keeps this variable — there is no innodb_redo_log_capacity. [MB] */
  innodb_log_file_size?: number | string;
  /** Log buffer size. In-memory redo buffer. [MB] */
  innodb_log_buffer_size?: number | string;
  /** Flush log at commit. 1 = ACID, 2 = write on commit and fsync once a second, 0 = neither. */
  innodb_flush_log_at_trx_commit?: number | string;
  /** I/O capacity. Background flush budget in IOPS. [IOPS] */
  innodb_io_capacity?: number | string;
  /** I/O capacity max. Catch-up flush budget; upstream default is max(2000, 2 x innodb_io_capacity). [IOPS] */
  innodb_io_capacity_max?: number | string;
  /** Read I/O threads. Background read threads. */
  innodb_read_io_threads?: number | string;
  /** Write I/O threads. Background write threads. */
  innodb_write_io_threads?: number | string;
  /** Lock wait timeout. Row-lock wait before the statement is rolled back. [s] */
  innodb_lock_wait_timeout?: number | string;
  /** File per table. Store each table in its own .ibd tablespace. */
  innodb_file_per_table?: "ON" | "OFF";
  /** Auto-increment lock mode. 0 = table lock, 1 = consecutive (the default), 2 = interleaved — the only mode Galera supports. */
  innodb_autoinc_lock_mode?: number | string;
  /** Binary log base name. Base name of the binary logs; binary logging is off by default in MariaDB, setting this enables it. */
  log_bin?: string;
  /** Binlog format. MariaDB's upstream default is MIXED; Galera requires ROW. */
  binlog_format?: "MIXED" | "ROW" | "STATEMENT";
  /** Binlog row image. How much of each row is logged. */
  binlog_row_image?: "FULL" | "MINIMAL" | "NOBLOB";
  /** Sync binlog. fsync the binary log every N commit groups; MariaDB's default is 0 (leave it to the OS). */
  sync_binlog?: number | string;
  /** Binlog retention. Purge age of the binary logs; 0 disables purging. Takes precedence over expire_logs_days. [s] */
  binlog_expire_logs_seconds?: number | string;
  /** Log slave updates. Write replicated events into this server's own binary log. */
  log_slave_updates?: "ON" | "OFF";
  /** GTID strict mode. Refuse out-of-order GTIDs; MariaDB has no gtid_mode — GTIDs always exist. */
  gtid_strict_mode?: "ON" | "OFF";
  /** GTID domain id. Replication domain this server writes to; a multi-source or multi-writer setup needs distinct domains. */
  gtid_domain_id?: number | string;
  /** Ignore duplicate GTIDs. Skip events whose GTID was already applied in that domain. */
  gtid_ignore_duplicates?: "ON" | "OFF";
  /** Semisync master. Semisynchronous primary side. MariaDB keeps the master/slave names and needs no plugin_load. */
  rpl_semi_sync_master_enabled?: "ON" | "OFF";
  /** Semisync timeout. Wait for a replica ack before falling back to asynchronous. [ms] */
  rpl_semi_sync_master_timeout?: number | string;
  /** Semisync wait point. AFTER_SYNC acks before the storage commit, AFTER_COMMIT after it. */
  rpl_semi_sync_master_wait_point?: "AFTER_COMMIT" | "AFTER_SYNC";
  /** Semisync slave. Semisynchronous replica side. */
  rpl_semi_sync_slave_enabled?: "ON" | "OFF";
  /** Parallel applier threads. Applier worker threads; 0 = single-threaded. MariaDB has no replica_parallel_workers. */
  slave_parallel_threads?: number | string;
  /** Parallel applier mode. How aggressively the applier parallelizes. */
  slave_parallel_mode?: "optimistic" | "conservative" | "aggressive" | "minimal" | "none";
  /** Read only. Reject writes from clients without SUPER — set on replicas. */
  read_only?: "ON" | "OFF";
  /** Galera enabled. Enable write-set replication; the whole wsrep_* block below is only rendered when this is ON. */
  wsrep_on?: "ON" | "OFF";
  /** Galera provider. Path to the Galera library. */
  wsrep_provider?: string;
  /** Cluster name. Logical cluster name; must be identical on every node. */
  wsrep_cluster_name?: string;
  /** SST method. How a joiner gets a full state snapshot. mariabackup is non-blocking; rsync blocks the donor. */
  wsrep_sst_method?: "mariabackup" | "rsync" | "mysqldump";
  /** SST credentials. user:password for backup SST; filled by the server from topology, unused by rsync. */
  wsrep_sst_auth?: string | null;
  /** Galera applier threads. Threads applying write-sets from other nodes. */
  wsrep_slave_threads?: number | string;
  /** Sync wait bitmask. Bitmask of statement classes that wait for the node to catch up first; 0 = never wait, 1 = SELECT. */
  wsrep_sync_wait?: number | string;
  /** Bind address. Address mariadbd listens on. */
  bind_address?: string;
  /** Port. TCP port. */
  port?: number | string;
  /** Max connections. Maximum simultaneous client connections. */
  max_connections?: number | string;
  /** Max connect errors. Failed handshakes from one host before it is blocked. */
  max_connect_errors?: number | string;
  /** Thread cache size. Threads kept for reuse. */
  thread_cache_size?: number | string;
  /** Wait timeout. Idle seconds before a non-interactive connection is closed. [s] */
  wait_timeout?: number | string;
  /** Interactive timeout. Idle seconds before an interactive connection is closed. [s] */
  interactive_timeout?: number | string;
  /** Max allowed packet. Largest single packet. [MB] */
  max_allowed_packet?: number | string;
  /** Table open cache. Open table handles cached across sessions. */
  table_open_cache?: number | string;
  /** Table definition cache. Cached table definitions. */
  table_definition_cache?: number | string;
  /** Temp table size. Largest in-memory internal temporary table. [MB] */
  tmp_table_size?: number | string;
  /** Max heap table size. Largest MEMORY table; also caps tmp_table_size. [MB] */
  max_heap_table_size?: number | string;
  /** Open files limit. File descriptors requested from the OS. */
  open_files_limit?: number | string;
  /** SQL mode. MariaDB's default (unchanged since 10.2.4 and identical in 10.11 and 11.4) still contains NO_AUTO_CREATE_USER, which MySQL 8 removed. */
  sql_mode?: string;
  /** Server character set. MariaDB's upstream default is latin1 until 11.6; stroppy pins utf8mb4 so results are comparable with MySQL runs. */
  character_set_server?: "utf8mb4" | "utf8mb3" | "latin1";
  /** Server collation. Must belong to character_set_server. MariaDB's own default is latin1_swedish_ci up to 11.7 and utf8mb4_uca1400_ai_ci from 11.8; stroppy keeps utf8mb4_general_ci on every line so MariaDB runs stay comparable with each other and with MySQL. */
  collation_server?: string;
  /** Default time zone. Server time zone; a fixed offset keeps benchmark timestamps reproducible. */
  default_time_zone?: string;
  /** Lower case table names. 0 = as stored, 1 = lowercased, 2 = stored as given and compared lowercase. Fixed at datadir initialization. */
  lower_case_table_names?: number | string;
  /** Performance schema. Instrumentation engine; off by default in MariaDB. */
  performance_schema?: "ON" | "OFF";
  /** General log. Log every statement — far too expensive during a load test. */
  general_log?: "ON" | "OFF";
  /** Slow query log. Log statements slower than log_slow_query_time. Renamed from slow_query_log in 10.11. */
  log_slow_query?: "ON" | "OFF";
  /** Slow query threshold. Slow-query threshold in seconds; fractions allowed. [s] */
  log_slow_query_time?: number;
  /** Slow log verbosity. Comma-separated extras in the slow log (query_plan, innodb, explain, engine, warnings, full, all); empty by default. */
  log_slow_verbosity?: string;
  /** Warning verbosity. Error-log verbosity. MariaDB has no log_error_verbosity; this is its equivalent. */
  log_warnings?: number | string;
  /** Change buffering. Which secondary-index operations are change-buffered. Deprecated and ignored since 10.9, removed in 11.0. */
  innodb_change_buffering?: "none" | "inserts" | "deletes" | "changes" | "purges" | "all";
  /** Doublewrite buffer. Torn-page protection. */
  innodb_doublewrite?: "ON" | "OFF";
  /** Flush method. Data/log flush method; MariaDB's default has been O_DIRECT since 10.6. */
  innodb_flush_method?: "O_DIRECT" | "fsync" | "O_DSYNC" | "O_DIRECT_NO_FSYNC";
  /** Transaction isolation. Default isolation level. SQL uses tx_isolation; the option file renders transaction-isolation. */
  tx_isolation?: "READ-UNCOMMITTED" | "READ-COMMITTED" | "REPEATABLE-READ" | "SERIALIZABLE";
  /** Server id. Unique replication id; filled by the server from topology. */
  server_id?: number | string | null;
  /** Report host. Address this replica reports to its primary; filled by the server from topology. */
  report_host?: string | null;
  /** Galera cluster address. gcomm:// URL listing the cluster members; filled by the server from topology. */
  wsrep_cluster_address?: string | null;
  /** Galera node address. This node's ip[:port] for group communication; filled by the server from topology. */
  wsrep_node_address?: string | null;
  /** Galera node name. This node's name inside the cluster; filled by the server from topology. */
  wsrep_node_name?: string | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Cluster lines. Rendered topology lines (server_id, report_host). */
  readonly cluster_lines?: string;
  /** Galera lines. The whole wsrep_* block, emitted only when wsrep_on is ON. */
  readonly galera_lines?: string;
  /** Rendered extra settings. The `custom` map joined into config lines; the template prints it at the end of the file. */
  readonly custom_rendered?: string;
}

