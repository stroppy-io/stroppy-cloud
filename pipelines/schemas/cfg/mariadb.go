package cfg

import (
	"fmt"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// cfg.mariadb.cnf@10.11 / @11.4 / @11.8 — the three MariaDB LTS lines.
//
// doc: https://mariadb.com/docs/server/server-management/variables-and-modes/server-system-variables
// doc: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-system-variables
// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables
// doc: https://mariadb.com/docs/galera-cluster/reference/galera-cluster-system-variables
//
// MariaDB is NOT MySQL here, and the differences are load-bearing:
//   - semisync keeps the master/slave names and is built into the server since
//     10.3.3, so there is no plugin_load_add;
//   - there is no gtid_mode — GTIDs are always on, tuned by gtid_strict_mode
//     and gtid_domain_id;
//   - log_slave_updates, slave_parallel_threads, wsrep_slave_threads keep the
//     old names (wsrep_applier_threads is a Percona XtraDB Cluster variable);
//   - innodb_thread_concurrency and innodb_buffer_pool_instances were removed
//     in 10.6, so neither major carries them;
//   - there is no log_error_verbosity — MariaDB uses log_warnings;
//   - since 10.11 the canonical slow-log names are log_slow_query*,
//     slow_query_log/long_query_time are the aliases.

// MariadbCnf1011 is cfg.mariadb.cnf@1011 (MariaDB 10.11 LTS).
func MariadbCnf1011() *schemapb.Schema { return mariadbCnf("10.11") }

// MariadbCnf1104 is cfg.mariadb.cnf@11.4 (MariaDB 11.4 LTS).
func MariadbCnf1104() *schemapb.Schema { return mariadbCnf("11.4") }

// MariadbCnf1108 is cfg.mariadb.cnf@11.8 (MariaDB 11.8 LTS).
//
// 11.8 is 11.4 plus the 11.5-11.7 rolling releases; nothing on the 11.4
// surface was removed or renamed. What actually changed, all verified against
// the 11.4 -> 11.8 upgrade page,
// https://mariadb.com/docs/server/server-management/install-and-upgrade-mariadb/upgrading/mariadb-community-server-upgrade-paths/upgrading-from-mariadb-11-4-to-mariadb-11-8:
//
//   - innodb_snapshot_isolation flipped OFF -> ON (MDEV-35124), which turns
//     write-write conflicts under REPEATABLE READ into error 1020 — modeled
//     explicitly so a run cannot inherit it by accident;
//   - the server character set / collation defaults became utf8mb4 /
//     utf8mb4_uca1400_ai_ci (MDEV-25829); the product already pinned utf8mb4;
//   - new knobs worth having in a benchmark file: max_tmp_session_space_usage
//     and max_tmp_total_space_usage (11.5), log_slow_always_query_time and
//     slave_abort_blocking_timeout (11.7).
func MariadbCnf1108() *schemapb.Schema { return mariadbCnf("11.8") }

//nolint:funlen,maintidx // one flat schema definition
func mariadbCnf(major string) *schemapb.Schema {
	is1011 := major == "10.11"
	is118 := major == "11.8"

	fields := []schemapb.FieldDef{
		// ---- InnoDB ------------------------------------------------------
		// doc: innodb-system-variables#innodb_buffer_pool_size
		schemapb.Int64("innodb_buffer_pool_size").Title("Buffer pool size").Group("InnoDB").
			Desc("InnoDB buffer pool. Upstream default 128M.").
			Unit("MB").Gte(5).Lte(4194304).Default(128),
		// doc: innodb-system-variables#innodb_log_file_size (still current in MariaDB; dynamic since 10.9)
		schemapb.Int64("innodb_log_file_size").Title("Redo log size").Group("InnoDB").
			Desc("Size of the redo log. Unlike MySQL 8.0.30+, MariaDB keeps this variable — there is no innodb_redo_log_capacity.").
			Unit("MB").Gte(1).Lte(524288).Default(96),
		// doc: innodb-system-variables#innodb_log_buffer_size
		schemapb.Int64("innodb_log_buffer_size").Title("Log buffer size").Group("InnoDB").
			Desc("In-memory redo buffer.").Unit("MB").Gte(1).Lte(2047).Default(16),
		// doc: innodb-system-variables#innodb_flush_log_at_trx_commit
		schemapb.Int64("innodb_flush_log_at_trx_commit").Title("Flush log at commit").Group("InnoDB").
			Desc("1 = ACID, 2 = write on commit and fsync once a second, 0 = neither.").
			In(0, 1, 2).Default(1),
		// doc: innodb-system-variables#innodb_io_capacity
		schemapb.Int64("innodb_io_capacity").Title("I/O capacity").Group("InnoDB").
			Desc("Background flush budget in IOPS.").Unit("IOPS").Gte(100).Lte(1000000).Default(200),
		// doc: innodb-system-variables#innodb_io_capacity_max
		schemapb.Int64("innodb_io_capacity_max").Title("I/O capacity max").Group("InnoDB").
			Desc("Catch-up flush budget; upstream default is max(2000, 2 x innodb_io_capacity).").
			Unit("IOPS").Gte(100).Lte(2000000).Default(2000),
		// doc: innodb-system-variables#innodb_read_io_threads (dynamic since 10.11)
		schemapb.Int64("innodb_read_io_threads").Title("Read I/O threads").Group("InnoDB").
			Desc("Background read threads.").Gte(1).Lte(64).Default(4),
		// doc: innodb-system-variables#innodb_write_io_threads
		schemapb.Int64("innodb_write_io_threads").Title("Write I/O threads").Group("InnoDB").
			Desc("Background write threads.").Gte(1).Lte(64).Default(4),
		// doc: innodb-system-variables#innodb_lock_wait_timeout
		schemapb.Int64("innodb_lock_wait_timeout").Title("Lock wait timeout").Group("InnoDB").
			Desc("Row-lock wait before the statement is rolled back.").
			Unit("s").Gte(1).Lte(1073741824).Default(50),
		// doc: innodb-system-variables#innodb_file_per_table
		myOnOff("innodb_file_per_table").Title("File per table").Group("InnoDB").
			Desc(mariadbFilePerTableDesc(is1011)).Default(myOn()),
		// doc: innodb-system-variables#innodb_autoinc_lock_mode (Galera requires 2)
		schemapb.Int64("innodb_autoinc_lock_mode").Title("Auto-increment lock mode").Group("InnoDB").
			Desc("0 = table lock, 1 = consecutive (the default), 2 = interleaved — the only mode Galera supports.").
			In(0, 1, 2).Default(1),

		// ---- Binary log ---------------------------------------------------
		// doc: replication-and-binary-log-system-variables#log_bin
		schemapb.Str("log_bin").Title("Binary log base name").Group("Binlog").
			Desc("Base name of the binary logs; binary logging is off by default in MariaDB, setting this enables it.").
			MinLen(1).MaxLen(200).Default("binlog"),
		// doc: replication-and-binary-log-system-variables#binlog_format (MariaDB default MIXED)
		schemapb.Choice("binlog_format").Title("Binlog format").Group("Binlog").
			Desc("MariaDB's upstream default is MIXED; Galera requires ROW.").
			Opt(schemapb.StrV("MIXED"), "MIXED").
			Opt(schemapb.StrV("ROW"), "ROW").
			Opt(schemapb.StrV("STATEMENT"), "STATEMENT").
			Default(schemapb.StrV("MIXED")),
		// doc: replication-and-binary-log-system-variables#binlog_row_image
		schemapb.Choice("binlog_row_image").Title("Binlog row image").Group("Binlog").
			Desc("How much of each row is logged.").
			Opt(schemapb.StrV("FULL"), "FULL").
			Opt(schemapb.StrV("MINIMAL"), "MINIMAL").
			Opt(schemapb.StrV("NOBLOB"), "NOBLOB").
			Default(schemapb.StrV("FULL")),
		// doc: replication-and-binary-log-system-variables#sync_binlog (MariaDB default 0)
		schemapb.Int64("sync_binlog").Title("Sync binlog").Group("Binlog").
			Desc("fsync the binary log every N commit groups; MariaDB's default is 0 (leave it to the OS).").
			Gte(0).Lte(4294967295).Default(0),
		// doc: replication-and-binary-log-system-variables#binlog_expire_logs_seconds (10.6.1+)
		schemapb.Int64("binlog_expire_logs_seconds").Title("Binlog retention").Group("Binlog").
			Desc("Purge age of the binary logs; 0 disables purging. Takes precedence over expire_logs_days.").
			Unit("s").Gte(0).Lte(4294967295).Default(0),
		// doc: replication-and-binary-log-system-variables#log_slave_updates (MariaDB keeps the slave name)
		myOnOff("log_slave_updates").Title("Log slave updates").Group("Binlog").
			Desc("Write replicated events into this server's own binary log.").Default(myOff()),

		// ---- GTID -----------------------------------------------------------
		// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid
		myOnOff("gtid_strict_mode").Title("GTID strict mode").Group("GTID").
			Desc("Refuse out-of-order GTIDs; MariaDB has no gtid_mode — GTIDs always exist.").Default(myOff()),
		// doc: gtid#gtid_domain_id
		schemapb.Int64("gtid_domain_id").Title("GTID domain id").Group("GTID").
			Desc("Replication domain this server writes to; a multi-source or multi-writer setup needs distinct domains.").
			Gte(0).Lte(4294967295).Default(0),
		// doc: gtid#gtid_ignore_duplicates
		myOnOff("gtid_ignore_duplicates").Title("Ignore duplicate GTIDs").Group("GTID").
			Desc("Skip events whose GTID was already applied in that domain.").Default(myOff()),

		// ---- Replication -----------------------------------------------------
		// doc: semisynchronous-replication — built into the server since 10.3.3, no plugin load
		myOnOff("rpl_semi_sync_master_enabled").Title("Semisync master").Group("Replication").
			Desc("Semisynchronous primary side. MariaDB keeps the master/slave names and needs no plugin_load.").
			Default(myOff()),
		// doc: semisynchronous-replication#rpl_semi_sync_master_timeout
		schemapb.Int64("rpl_semi_sync_master_timeout").Title("Semisync timeout").Group("Replication").
			Desc("Wait for a replica ack before falling back to asynchronous.").
			Unit("ms").Gte(0).Lte(4294967295).Default(10000),
		// doc: semisynchronous-replication#rpl_semi_sync_master_wait_point
		schemapb.Choice("rpl_semi_sync_master_wait_point").Title("Semisync wait point").Group("Replication").
			Desc("AFTER_SYNC acks before the storage commit, AFTER_COMMIT after it.").
			Opt(schemapb.StrV("AFTER_COMMIT"), "AFTER_COMMIT").
			Opt(schemapb.StrV("AFTER_SYNC"), "AFTER_SYNC").
			Default(schemapb.StrV("AFTER_COMMIT")),
		// doc: semisynchronous-replication#rpl_semi_sync_slave_enabled
		myOnOff("rpl_semi_sync_slave_enabled").Title("Semisync slave").Group("Replication").
			Desc("Semisynchronous replica side.").Default(myOff()),
		// doc: replication-and-binary-log-system-variables#slave_parallel_threads
		schemapb.Int64("slave_parallel_threads").Title("Parallel applier threads").Group("Replication").
			Desc("Applier worker threads; 0 = single-threaded. MariaDB has no replica_parallel_workers.").
			Gte(0).Lte(16383).Default(0),
		// doc: replication-and-binary-log-system-variables#slave_parallel_mode (default optimistic since 10.5.1)
		schemapb.Choice("slave_parallel_mode").Title("Parallel applier mode").Group("Replication").
			Desc("How aggressively the applier parallelizes.").
			Opt(schemapb.StrV("optimistic"), "optimistic").
			Opt(schemapb.StrV("conservative"), "conservative").
			Opt(schemapb.StrV("aggressive"), "aggressive").
			Opt(schemapb.StrV("minimal"), "minimal").
			Opt(schemapb.StrV("none"), "none").
			Default(schemapb.StrV("optimistic")),
		// doc: server-system-variables#read_only
		myOnOff("read_only").Title("Read only").Group("Replication").
			Desc("Reject writes from clients without SUPER — set on replicas.").Default(myOff()),

		// ---- Galera ------------------------------------------------------------
		// doc: galera-cluster-system-variables#wsrep_on
		myOnOff("wsrep_on").Title("Galera enabled").Group("Galera").
			Desc("Enable write-set replication; the whole wsrep_* block below is only rendered when this is ON.").
			Default(myOff()),
		// doc: galera-cluster-system-variables#wsrep_provider
		schemapb.Str("wsrep_provider").Title("Galera provider").Group("Galera").
			Desc("Path to the Galera library.").
			MinLen(1).MaxLen(255).Default("/usr/lib/galera/libgalera_smm.so"),
		// doc: galera-cluster-system-variables#wsrep_cluster_name
		schemapb.Str("wsrep_cluster_name").Title("Cluster name").Group("Galera").
			Desc("Logical cluster name; must be identical on every node.").
			MinLen(1).MaxLen(64).Default("stroppy"),
		// doc: galera-cluster-system-variables#wsrep_sst_method
		schemapb.Choice("wsrep_sst_method").Title("SST method").Group("Galera").
			Desc("How a joiner gets a full state snapshot. mariabackup is non-blocking; rsync blocks the donor.").
			Opt(schemapb.StrV("mariabackup"), "mariabackup").
			Opt(schemapb.StrV("rsync"), "rsync").
			Opt(schemapb.StrV("mysqldump"), "mysqldump").
			Default(schemapb.StrV("mariabackup")),
		// doc: https://mariadb.com/docs/galera-cluster/reference/galera-cluster-system-variables#wsrep_sst_auth
		schemapb.Str("wsrep_sst_auth").Title("SST credentials").Group("Cluster").
			Desc("user:password for backup SST; filled by the server from topology, unused by rsync.").
			Secret().MaxLen(255).Nullable(),
		// doc: galera-cluster-system-variables#wsrep_slave_threads
		// (MariaDB keeps this name in both 10.11 and 11.4; wsrep_applier_threads is Percona XtraDB Cluster only)
		schemapb.Int64("wsrep_slave_threads").Title("Galera applier threads").Group("Galera").
			Desc("Threads applying write-sets from other nodes.").Gte(1).Lte(512).Default(1),
		// doc: galera-cluster-system-variables#wsrep_sync_wait
		schemapb.Int64("wsrep_sync_wait").Title("Sync wait bitmask").Group("Galera").
			Desc("Bitmask of statement classes that wait for the node to catch up first; 0 = never wait, 1 = SELECT.").
			Gte(0).Lte(15).Default(0),

		// ---- Connections ---------------------------------------------------------
		// doc: server-system-variables#bind_address
		schemapb.Str("bind_address").Title("Bind address").Group("Connections").
			Desc("Address mariadbd listens on.").MinLen(1).MaxLen(255).Default("0.0.0.0"),
		// doc: server-system-variables#port
		schemapb.Int64("port").Title("Port").Group("Connections").
			Desc("TCP port.").Gte(1).Lte(65535).Default(3306),
		// doc: server-system-variables#max_connections
		schemapb.Int64("max_connections").Title("Max connections").Group("Connections").
			Desc("Maximum simultaneous client connections.").Gte(1).Lte(100000).Default(151),
		// doc: server-system-variables#max_connect_errors
		schemapb.Int64("max_connect_errors").Title("Max connect errors").Group("Connections").
			Desc("Failed handshakes from one host before it is blocked.").
			Gte(1).Lte(4294967295).Default(100),
		// doc: server-system-variables#thread_cache_size (MariaDB default 256)
		schemapb.Int64("thread_cache_size").Title("Thread cache size").Group("Connections").
			Desc("Threads kept for reuse.").Gte(0).Lte(16384).Default(256),
		// doc: server-system-variables#wait_timeout
		schemapb.Int64("wait_timeout").Title("Wait timeout").Group("Connections").
			Desc("Idle seconds before a non-interactive connection is closed.").
			Unit("s").Gte(1).Lte(31536000).Default(28800),
		// doc: server-system-variables#interactive_timeout
		schemapb.Int64("interactive_timeout").Title("Interactive timeout").Group("Connections").
			Desc("Idle seconds before an interactive connection is closed.").
			Unit("s").Gte(1).Lte(31536000).Default(28800),
		// doc: server-system-variables#max_allowed_packet
		schemapb.Int64("max_allowed_packet").Title("Max allowed packet").Group("Connections").
			Desc("Largest single packet.").Unit("MB").Gte(1).Lte(1024).Default(16),

		// ---- Tables / cache --------------------------------------------------------
		// doc: server-system-variables#table_open_cache (MariaDB default 2000)
		schemapb.Int64("table_open_cache").Title("Table open cache").Group("Tables").
			Desc("Open table handles cached across sessions.").Gte(1).Lte(1048576).Default(2000),
		// doc: server-system-variables#table_definition_cache (MariaDB default 400)
		schemapb.Int64("table_definition_cache").Title("Table definition cache").Group("Tables").
			Desc("Cached table definitions.").Gte(256).Lte(524288).Default(400),
		// doc: server-system-variables#tmp_table_size
		schemapb.Int64("tmp_table_size").Title("Temp table size").Group("Tables").
			Desc("Largest in-memory internal temporary table.").Unit("MB").Gte(1).Lte(65536).Default(16),
		// doc: server-system-variables#max_heap_table_size
		schemapb.Int64("max_heap_table_size").Title("Max heap table size").Group("Tables").
			Desc("Largest MEMORY table; also caps tmp_table_size.").Unit("MB").Gte(1).Lte(65536).Default(16),
		// doc: server-system-variables#open_files_limit
		schemapb.Int64("open_files_limit").Title("Open files limit").Group("Tables").
			Desc("File descriptors requested from the OS.").Gte(1024).Lte(1048576).Default(32768),

		// ---- SQL ---------------------------------------------------------------------
		// doc: https://mariadb.com/docs/server/server-management/variables-and-modes/sql_mode
		schemapb.Str("sql_mode").Title("SQL mode").Group("SQL").
			Desc("MariaDB's default (unchanged since 10.2.4 and identical in 10.11 and 11.4) still contains NO_AUTO_CREATE_USER, which MySQL 8 removed.").
			MaxLen(1024).
			Default("STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION"),
		// doc: server-system-variables#character_set_server
		schemapb.Choice("character_set_server").Title("Server character set").Group("SQL").
			Desc("MariaDB's upstream default is latin1 until 11.6; stroppy pins utf8mb4 so results are comparable with MySQL runs.").
			Opt(schemapb.StrV("utf8mb4"), "utf8mb4").
			Opt(schemapb.StrV("utf8mb3"), "utf8mb3").
			Opt(schemapb.StrV("latin1"), "latin1").
			Default(schemapb.StrV("utf8mb4")),
		// doc: server-system-variables#collation_server
		schemapb.Str("collation_server").Title("Server collation").Group("SQL").
			Desc("Must belong to character_set_server. MariaDB's own default is latin1_swedish_ci up to 11.7 and utf8mb4_uca1400_ai_ci from 11.8; stroppy keeps utf8mb4_general_ci on every line so MariaDB runs stay comparable with each other and with MySQL.").
			MinLen(1).MaxLen(64).Default("utf8mb4_general_ci"),
		// doc: server-system-variables#default_time_zone
		schemapb.Str("default_time_zone").Title("Default time zone").Group("SQL").
			Desc("Server time zone; a fixed offset keeps benchmark timestamps reproducible.").
			MinLen(1).MaxLen(64).Default("+00:00"),
		// doc: server-system-variables#lower_case_table_names
		schemapb.Int64("lower_case_table_names").Title("Lower case table names").Group("SQL").
			Desc("0 = as stored, 1 = lowercased, 2 = stored as given and compared lowercase. Fixed at datadir initialization.").
			In(0, 1, 2).Default(0),

		// ---- Performance schema -------------------------------------------------------
		// doc: https://mariadb.com/docs/server/reference/system-tables/performance-schema
		myOnOff("performance_schema").Title("Performance schema").Group("Performance schema").
			Desc("Instrumentation engine; off by default in MariaDB.").Default(myOff()),

		// ---- Logging --------------------------------------------------------------------
		// doc: server-system-variables#general_log
		myOnOff("general_log").Title("General log").Group("Logging").
			Desc("Log every statement — far too expensive during a load test.").Default(myOff()),
		// doc: server-system-variables#log_slow_query (the canonical name since 10.11; slow_query_log is the alias)
		myOnOff("log_slow_query").Title("Slow query log").Group("Logging").
			Desc("Log statements slower than log_slow_query_time. Renamed from slow_query_log in 10.11.").
			Default(myOff()),
		// doc: server-system-variables#log_slow_query_time (renamed from long_query_time in 10.11)
		schemapb.Double("log_slow_query_time").Title("Slow query threshold").Group("Logging").
			Desc("Slow-query threshold in seconds; fractions allowed.").
			Unit("s").Gte(0).Lte(31536000).Default(10),
		// doc: server-system-variables#log_slow_verbosity
		schemapb.Str("log_slow_verbosity").Title("Slow log verbosity").Group("Logging").
			Desc("Comma-separated extras in the slow log (query_plan, innodb, explain, engine, warnings, full, all); empty by default.").
			MaxLen(255).Default(""),
		// doc: server-system-variables#log_warnings — MariaDB has NO log_error_verbosity
		schemapb.Int64("log_warnings").Title("Warning verbosity").Group("Logging").
			Desc("Error-log verbosity. MariaDB has no log_error_verbosity; this is its equivalent.").
			Gte(0).Lte(11).Default(2),
	}

	// ---- version-gated fields ------------------------------------------------
	if is1011 {
		fields = append(fields,
			// doc: innodb-system-variables#innodb_change_buffering
			// (deprecated and ignored in 10.9, REMOVED in 11.0 — 10.11 only)
			schemapb.Choice("innodb_change_buffering").Title("Change buffering").Group("InnoDB").
				Desc("Which secondary-index operations are change-buffered. Deprecated and ignored since 10.9, removed in 11.0.").
				Opt(schemapb.StrV("none"), "none").
				Opt(schemapb.StrV("inserts"), "inserts").
				Opt(schemapb.StrV("deletes"), "deletes").
				Opt(schemapb.StrV("changes"), "changes").
				Opt(schemapb.StrV("purges"), "purges").
				Opt(schemapb.StrV("all"), "all").
				Default(schemapb.StrV("none")).Deprecated(),
			// doc: innodb-system-variables#innodb_doublewrite (boolean in 10.11)
			myOnOff("innodb_doublewrite").Title("Doublewrite buffer").Group("InnoDB").
				Desc("Torn-page protection.").Default(myOn()),
			// doc: innodb-system-variables#innodb_flush_method (deprecated only from 11.0)
			schemapb.Choice("innodb_flush_method").Title("Flush method").Group("InnoDB").
				Desc("Data/log flush method; MariaDB's default has been O_DIRECT since 10.6.").
				Opt(schemapb.StrV("O_DIRECT"), "O_DIRECT").
				Opt(schemapb.StrV("fsync"), "fsync").
				Opt(schemapb.StrV("O_DSYNC"), "O_DSYNC").
				Opt(schemapb.StrV("O_DIRECT_NO_FSYNC"), "O_DIRECT_NO_FSYNC").
				Default(schemapb.StrV("O_DIRECT")),
			// doc: server-system-variables#tx_isolation
			// SQL uses tx_isolation in 10.11, but the startup option is transaction-isolation.
			schemapb.Choice("tx_isolation").Title("Transaction isolation").Group("SQL").
				Desc("Default isolation level. SQL uses tx_isolation; the option file renders transaction-isolation.").
				Opt(schemapb.StrV("READ-UNCOMMITTED"), "READ UNCOMMITTED").
				Opt(schemapb.StrV("READ-COMMITTED"), "READ COMMITTED").
				Opt(schemapb.StrV("REPEATABLE-READ"), "REPEATABLE READ").
				Opt(schemapb.StrV("SERIALIZABLE"), "SERIALIZABLE").
				Default(schemapb.StrV("REPEATABLE-READ")),
		)
	} else {
		fields = append(fields,
			// doc: innodb-system-variables#innodb_doublewrite (enum OFF/ON/fast since 11.0.6)
			schemapb.Choice("innodb_doublewrite").Title("Doublewrite buffer").Group("InnoDB").
				Desc("Torn-page protection. 11.x adds `fast`, which skips the doublewrite fsync on already-durable storage.").
				Opt(schemapb.StrV("ON"), "on").
				Opt(schemapb.StrV("OFF"), "off").
				Opt(schemapb.StrV("fast"), "fast").
				Default(schemapb.StrV("ON")),
			// doc: innodb-system-variables#innodb_log_file_buffering (the 11.0 successor of innodb_flush_method)
			myOnOff("innodb_log_file_buffering").Title("Buffer the redo log").Group("InnoDB").
				Desc("Let the OS page cache buffer redo writes. Part of the set that replaced innodb_flush_method in 11.0.").
				Default(myOff()),
			// doc: innodb-system-variables#innodb_data_file_buffering
			myOnOff("innodb_data_file_buffering").Title("Buffer the data files").Group("InnoDB").
				Desc("Let the OS page cache buffer data-file writes; OFF is the O_DIRECT equivalent.").
				Default(myOff()),
			// doc: server-system-variables#transaction_isolation (11.1.1+)
			schemapb.Choice("transaction_isolation").Title("Transaction isolation").Group("SQL").
				Desc("Default isolation level; the MySQL-compatible name, available since 11.1. tx_isolation still works but is deprecated.").
				Opt(schemapb.StrV("READ-UNCOMMITTED"), "READ UNCOMMITTED").
				Opt(schemapb.StrV("READ-COMMITTED"), "READ COMMITTED").
				Opt(schemapb.StrV("REPEATABLE-READ"), "REPEATABLE READ").
				Opt(schemapb.StrV("SERIALIZABLE"), "SERIALIZABLE").
				Default(schemapb.StrV("REPEATABLE-READ")),
		)
	}

	if is118 {
		fields = append(fields,
			// doc: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-system-variables#innodb_snapshot_isolation
			// (MDEV-35124: exists since 11.4.2 with default OFF, default flipped to ON in 11.8)
			myOnOff("innodb_snapshot_isolation").Title("Snapshot isolation").Group("InnoDB").
				Desc("REPEATABLE READ becomes true snapshot isolation: a write-write conflict raises error 1020 instead of silently reading an older row. 11.8 turned this ON by default; OFF restores the 11.4 behavior.").
				Default(myOn()),
			// doc: https://mariadb.com/docs/server/server-management/variables-and-modes/server-system-variables#max_tmp_session_space_usage (11.5)
			schemapb.Int64("max_tmp_session_space_usage").Title("Temp space per session").Group("Tables and caches").
				Desc("Cap on the temporary tables and files one session may occupy on disk; 0 disables the cap. New in 11.5.").
				Unit("MB").Gte(0).Lte(4194304).Default(0),
			// doc: server-system-variables#max_tmp_total_space_usage (11.5)
			schemapb.Int64("max_tmp_total_space_usage").Title("Temp space total").Group("Tables and caches").
				Desc("Cap on the temporary disk space of the whole server; 0 disables the cap. New in 11.5.").
				Unit("MB").Gte(0).Lte(16777216).Default(0),
			// doc: server-system-variables#log_slow_always_query_time (11.7)
			schemapb.Int64("log_slow_always_query_time").Title("Always-log slow time").Group("Logging").
				Desc("Queries slower than this are written to the slow log whatever the other log_slow_* filters say. New in 11.7; 0 means the filters decide.").
				Unit("s").Gte(0).Lte(31536000).Default(0),
			// doc: replication-and-binary-log-system-variables#slave_abort_blocking_timeout (11.7)
			schemapb.Int64("slave_abort_blocking_timeout").Title("Replica abort blocking timeout").Group("Replication").
				Desc("How long the replica waits for a local query that blocks replication before killing it. New in 11.7; the upstream default of 31536000 s (a year) means never.").
				Unit("s").Gte(0).Lte(31536000).Default(31536000),
		)
	}

	// ---- cluster-filled --------------------------------------------------------
	fields = append(fields,
		schemapb.Int64("server_id").Title("Server id").Group("Cluster").
			Desc("Unique replication id; filled by the server from topology.").
			Gte(1).Lte(4294967295).Nullable(),
		schemapb.Str("report_host").Title("Report host").Group("Cluster").
			Desc("Address this replica reports to its primary; filled by the server from topology.").
			MaxLen(255).Nullable(),
		schemapb.Str("wsrep_cluster_address").Title("Galera cluster address").Group("Cluster").
			Desc("gcomm:// URL listing the cluster members; filled by the server from topology.").
			MaxLen(4096).Nullable(),
		schemapb.Str("wsrep_node_address").Title("Galera node address").Group("Cluster").
			Desc("This node's ip[:port] for group communication; filled by the server from topology.").
			MaxLen(255).Nullable(),
		schemapb.Str("wsrep_node_name").Title("Galera node name").Group("Cluster").
			Desc("This node's name inside the cluster; filled by the server from topology.").
			MaxLen(255).Nullable(),

		customField().MaxEntries(128),

		schemapb.Computed("cluster_lines", myJoin(
			myEmit("server_id", "server_id"),
			myEmit("report_host", "report_host"),
		)).Result(schemapb.ResultString).Group("Rendered").
			Title("Cluster lines").Desc("Rendered topology lines (server_id, report_host)."),

		schemapb.Computed("galera_lines", mariadbGaleraLines()).
			Result(schemapb.ResultString).Group("Rendered").
			Title("Galera lines").Desc("The whole wsrep_* block, emitted only when wsrep_on is ON."),

		customRendered(),
	)

	return schemapb.NewSchema(mariadbID(major)).
		Descr("MariaDB "+major+" server configuration rendered to /etc/mysql/mariadb.conf.d/50-server.cnf ([mariadb] section).").
		Strict().Coerce().
		Fields(fields...).
		Rules(
			schemapb.Rule(`!("innodb_io_capacity" in root) || !("innodb_io_capacity_max" in root) || int(root.innodb_io_capacity_max) >= int(root.innodb_io_capacity)`,
				"innodb_io_capacity_max must be >= innodb_io_capacity").ID("io-capacity-order"),
			schemapb.Rule(mariadbGalera(`!("binlog_format" in root) || root.binlog_format == "ROW"`),
				"Galera requires binlog_format = ROW").ID("galera-binlog-format"),
			schemapb.Rule(mariadbGalera(`("innodb_autoinc_lock_mode" in root) && int(root.innodb_autoinc_lock_mode) == 2`),
				"Galera requires innodb_autoinc_lock_mode = 2").ID("galera-autoinc-lock-mode"),
			schemapb.Rule(mariadbGalera(`("wsrep_provider" in root) && string(root.wsrep_provider) != ""`),
				"Galera requires wsrep_provider").ID("galera-provider"),
			schemapb.Rule(`!("max_heap_table_size" in root) || !("tmp_table_size" in root) || int(root.max_heap_table_size) >= int(root.tmp_table_size)`,
				"max_heap_table_size must be >= tmp_table_size").ID("heap-ge-tmp"),
			customKeyRule(),
		).
		Template("conf", mariadbTemplate(major, is1011, is118)).
		MustBuild()
}

// mariadbGalera guards a Galera invariant against the raw (unresolved) values.
func mariadbGalera(expr string) string {
	return `!("wsrep_on" in root) || root.wsrep_on != "ON" || (` + expr + `)`
}

func mariadbFilePerTableDesc(is1011 bool) string {
	if is1011 {
		return "Store each table in its own .ibd tablespace."
	}

	return "Store each table in its own .ibd tablespace. Deprecated since 11.0.1 — the shared tablespace is on its way out."
}

// mariadbID maps the MariaDB LTS line to the schema identity: the public id
// keeps the major.minor form MariaDB itself uses (@10.11, @11.4, @11.8).
func mariadbID(major string) *schemapb.SchemaIdentity {
	switch major {
	case "10.11":
		return ids.CfgMinor("mariadb.cnf", 10, 11)
	case "11.8":
		return ids.CfgMinor("mariadb.cnf", 11, 8)
	default:
		return ids.CfgMinor("mariadb.cnf", 11, 4)
	}
}

func mariadbGaleraLines() string {
	body := myJoin(
		`"wsrep_on = ON\n"`,
		myEmit("wsrep_provider", "wsrep_provider"),
		myEmit("wsrep_cluster_name", "wsrep_cluster_name"),
		myEmit("wsrep_cluster_address", "wsrep_cluster_address"),
		myEmit("wsrep_node_address", "wsrep_node_address"),
		myEmit("wsrep_node_name", "wsrep_node_name"),
		myEmit("wsrep_sst_method", "wsrep_sst_method"),
		myEmit("wsrep_sst_auth", "wsrep_sst_auth"),
		myEmit("wsrep_slave_threads", "wsrep_slave_threads"),
		myEmit("wsrep_sync_wait", "wsrep_sync_wait"),
	)

	return `((("wsrep_on" in root) && root.wsrep_on == "ON") ? (` + body + `) : "wsrep_on = OFF\n")`
}

//nolint:funlen // one linear template build
func mariadbTemplate(major string, is1011, is118 bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# MariaDB %s — generated by stroppy, do not edit by hand\n[mariadb]\n\n", major)

	b.WriteString(`# InnoDB
innodb_buffer_pool_size = {{{values.innodb_buffer_pool_size}}}M
innodb_log_file_size = {{{values.innodb_log_file_size}}}M
innodb_log_buffer_size = {{{values.innodb_log_buffer_size}}}M
innodb_flush_log_at_trx_commit = {{{values.innodb_flush_log_at_trx_commit}}}
innodb_io_capacity = {{{values.innodb_io_capacity}}}
innodb_io_capacity_max = {{{values.innodb_io_capacity_max}}}
innodb_read_io_threads = {{{values.innodb_read_io_threads}}}
innodb_write_io_threads = {{{values.innodb_write_io_threads}}}
innodb_lock_wait_timeout = {{{values.innodb_lock_wait_timeout}}}
innodb_file_per_table = {{{values.innodb_file_per_table}}}
innodb_autoinc_lock_mode = {{{values.innodb_autoinc_lock_mode}}}
innodb_doublewrite = {{{values.innodb_doublewrite}}}
`)

	if is1011 {
		b.WriteString("innodb_flush_method = {{{values.innodb_flush_method}}}\ninnodb_change_buffering = {{{values.innodb_change_buffering}}}\n")
	} else {
		b.WriteString("innodb_log_file_buffering = {{{values.innodb_log_file_buffering}}}\ninnodb_data_file_buffering = {{{values.innodb_data_file_buffering}}}\n")
	}

	if is118 {
		b.WriteString("innodb_snapshot_isolation = {{{values.innodb_snapshot_isolation}}}\n")
	}

	b.WriteString(`
# Binary log
log_bin = {{{values.log_bin}}}
binlog_format = {{{values.binlog_format}}}
binlog_row_image = {{{values.binlog_row_image}}}
sync_binlog = {{{values.sync_binlog}}}
binlog_expire_logs_seconds = {{{values.binlog_expire_logs_seconds}}}
log_slave_updates = {{{values.log_slave_updates}}}
{{{values.cluster_lines}}}
# GTID
gtid_strict_mode = {{{values.gtid_strict_mode}}}
gtid_domain_id = {{{values.gtid_domain_id}}}
gtid_ignore_duplicates = {{{values.gtid_ignore_duplicates}}}

# Replication
rpl_semi_sync_master_enabled = {{{values.rpl_semi_sync_master_enabled}}}
rpl_semi_sync_master_timeout = {{{values.rpl_semi_sync_master_timeout}}}
rpl_semi_sync_master_wait_point = {{{values.rpl_semi_sync_master_wait_point}}}
rpl_semi_sync_slave_enabled = {{{values.rpl_semi_sync_slave_enabled}}}
slave_parallel_threads = {{{values.slave_parallel_threads}}}
slave_parallel_mode = {{{values.slave_parallel_mode}}}
read_only = {{{values.read_only}}}

# Galera
{{{values.galera_lines}}}
# Connections
bind_address = {{{values.bind_address}}}
port = {{{values.port}}}
max_connections = {{{values.max_connections}}}
max_connect_errors = {{{values.max_connect_errors}}}
thread_cache_size = {{{values.thread_cache_size}}}
wait_timeout = {{{values.wait_timeout}}}
interactive_timeout = {{{values.interactive_timeout}}}
max_allowed_packet = {{{values.max_allowed_packet}}}M

# Tables and caches
table_open_cache = {{{values.table_open_cache}}}
table_definition_cache = {{{values.table_definition_cache}}}
tmp_table_size = {{{values.tmp_table_size}}}M
max_heap_table_size = {{{values.max_heap_table_size}}}M
open_files_limit = {{{values.open_files_limit}}}

# SQL
sql_mode = {{{values.sql_mode}}}
character_set_server = {{{values.character_set_server}}}
collation_server = {{{values.collation_server}}}
default_time_zone = {{{values.default_time_zone}}}
lower_case_table_names = {{{values.lower_case_table_names}}}
`)

	if is1011 {
		b.WriteString("transaction-isolation = {{{values.tx_isolation}}}\n")
	} else {
		b.WriteString("transaction_isolation = {{{values.transaction_isolation}}}\n")
	}

	b.WriteString(`
# Performance schema
performance_schema = {{{values.performance_schema}}}

# Logging
general_log = {{{values.general_log}}}
log_slow_query = {{{values.log_slow_query}}}
log_slow_query_time = {{{values.log_slow_query_time}}}
log_slow_verbosity = {{{values.log_slow_verbosity}}}
log_warnings = {{{values.log_warnings}}}
`)

	if is118 {
		b.WriteString(`
# 11.5-11.7 additions
max_tmp_session_space_usage = {{{values.max_tmp_session_space_usage}}}M
max_tmp_total_space_usage = {{{values.max_tmp_total_space_usage}}}M
log_slow_always_query_time = {{{values.log_slow_always_query_time}}}
slave_abort_blocking_timeout = {{{values.slave_abort_blocking_timeout}}}
`)
	}

	b.WriteString(`
# Custom
{{{values.custom_rendered}}}
`)

	return b.String()
}
