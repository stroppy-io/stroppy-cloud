package cfg

import (
	"fmt"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// This file builds cfg.my.cnf@8.0 and cfg.my.cnf@8.4 from one builder.
// Every variable below is checked against the server system variable
// reference of the matching major:
//
//	8.0 https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html
//	8.4 https://dev.mysql.com/doc/refman/8.4/en/server-system-variables.html
//	    https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html (removals)
//
// Variables removed in 8.4 (transaction_write_set_extraction,
// binlog_transaction_dependency_tracking, replica_parallel_type,
// default_authentication_plugin) are only present in the 8.0 schema; the
// 8.4-only ones (authentication_policy, mysql_native_password) only there.

// myOnOff is an ON/OFF choice — the form MySQL writes boolean variables in.
func myOnOff(name schemapb.FieldName) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("ON"), "on").
		Opt(schemapb.StrV("OFF"), "off")
}

// myOn / myOff are the ON/OFF defaults.
func myOn() *schemapb.Value  { return schemapb.StrV("ON") }
func myOff() *schemapb.Value { return schemapb.StrV("OFF") }

// myEmit is the CEL fragment emitting `key = <root.field>\n` when the field is
// present, and nothing when it is not — the only way to keep an optional line
// out of a logic-less Mustache template.
func myEmit(key, field string) string {
	return fmt.Sprintf(`((%[2]q in root) ? %[1]q + " = " + string(root[%[2]q]) + "\n" : "")`, key, field)
}

// myGR guards a group-replication invariant: rules are evaluated against the
// raw (unresolved) values, so every access has to be presence-checked.
func myGR(expr string) string {
	return `!("group_replication" in root) || !root.group_replication || (` + expr + `)`
}

// myJoin sums CEL fragments.
func myJoin(parts ...string) string { return strings.Join(parts, " +\n") }

// MyCnf80 is cfg.my.cnf@8.0.
func MyCnf80() *schemapb.Schema { return myCnf("8.0") }

// MyCnf84 is cfg.my.cnf@8.4.
func MyCnf84() *schemapb.Schema { return myCnf("8.4") }

//nolint:funlen,maintidx // one flat schema definition
func myCnf(major string) *schemapb.Schema {
	is80 := major == "8.0"

	// 8.4 raised the InnoDB I/O defaults; see mysql-nutshell.html.
	ioCap, ioCapMax := int64(200), int64(2000)
	if !is80 {
		ioCap, ioCapMax = 10000, 20000
	}

	fields := []schemapb.FieldDef{
		// ---- InnoDB ------------------------------------------------------
		// doc: https://dev.mysql.com/doc/refman/8.0/en/innodb-parameters.html#sysvar_innodb_buffer_pool_size
		schemapb.Int64("innodb_buffer_pool_size").Title("Buffer pool size").Group("InnoDB").
			Desc("InnoDB buffer pool; the single most important memory setting. Upstream default 128M.").
			Unit("MB").Gte(5).Lte(4194304).Default(128),
		// doc: .../innodb-parameters.html#sysvar_innodb_buffer_pool_instances
		schemapb.Int64("innodb_buffer_pool_instances").Title("Buffer pool instances").Group("InnoDB").
			Desc("Number of buffer pool regions; only meaningful when the pool is larger than 1G.").
			Gte(1).Lte(64).Default(1),
		// doc: .../innodb-parameters.html#sysvar_innodb_redo_log_capacity (8.0.30+; supersedes innodb_log_file_size)
		schemapb.Int64("innodb_redo_log_capacity").Title("Redo log capacity").Group("InnoDB").
			Desc("Total redo log size. Replaces the deprecated innodb_log_file_size/innodb_log_files_in_group.").
			Unit("MB").Gte(8).Lte(131072).Default(100),
		// doc: .../innodb-parameters.html#sysvar_innodb_flush_log_at_trx_commit
		schemapb.Int64("innodb_flush_log_at_trx_commit").Title("Flush log at commit").Group("InnoDB").
			Desc("1 = ACID (flush+fsync on commit), 2 = flush on commit and fsync once a second, 0 = neither.").
			In(0, 1, 2).Default(1),
		// doc: .../innodb-parameters.html#sysvar_innodb_flush_method
		schemapb.Choice("innodb_flush_method").Title("Flush method").Group("InnoDB").
			Desc("Data/log flush method. Upstream default is fsync; stroppy defaults to O_DIRECT so the OS page cache does not double-buffer the pool during a benchmark.").
			Opt(schemapb.StrV("fsync"), "fsync").
			Opt(schemapb.StrV("O_DIRECT"), "O_DIRECT").
			Opt(schemapb.StrV("O_DIRECT_NO_FSYNC"), "O_DIRECT_NO_FSYNC").
			Opt(schemapb.StrV("O_DSYNC"), "O_DSYNC").
			Default(schemapb.StrV("O_DIRECT")),
		// doc: .../innodb-parameters.html#sysvar_innodb_io_capacity
		schemapb.Int64("innodb_io_capacity").Title("I/O capacity").Group("InnoDB").
			Desc("Background page-flush budget in IOPS. Upstream default " + fmt.Sprint(ioCap) + " for " + major + ".").
			Unit("IOPS").Gte(100).Lte(1000000).Default(ioCap),
		// doc: .../innodb-parameters.html#sysvar_innodb_io_capacity_max
		schemapb.Int64("innodb_io_capacity_max").Title("I/O capacity max").Group("InnoDB").
			Desc("Upper flush budget when InnoDB has to catch up; must be >= innodb_io_capacity.").
			Unit("IOPS").Gte(100).Lte(2000000).Default(ioCapMax),
		// doc: .../innodb-parameters.html#sysvar_innodb_read_io_threads
		schemapb.Int64("innodb_read_io_threads").Title("Read I/O threads").Group("InnoDB").
			Desc("Background read threads.").Gte(1).Lte(64).Default(4),
		// doc: .../innodb-parameters.html#sysvar_innodb_write_io_threads
		schemapb.Int64("innodb_write_io_threads").Title("Write I/O threads").Group("InnoDB").
			Desc("Background write threads.").Gte(1).Lte(64).Default(4),
		// doc: .../innodb-parameters.html#sysvar_innodb_thread_concurrency
		schemapb.Int64("innodb_thread_concurrency").Title("Thread concurrency").Group("InnoDB").
			Desc("Cap on threads inside InnoDB; 0 = unlimited (the default).").
			Gte(0).Lte(1000).Default(0),
		// doc: .../innodb-parameters.html#sysvar_innodb_lock_wait_timeout
		schemapb.Int64("innodb_lock_wait_timeout").Title("Lock wait timeout").Group("InnoDB").
			Desc("How long a transaction waits for a row lock before it is rolled back.").
			Unit("s").Gte(1).Lte(1073741824).Default(50),
		// doc: .../innodb-parameters.html#sysvar_innodb_doublewrite
		myOnOff("innodb_doublewrite").Title("Doublewrite buffer").Group("InnoDB").
			Desc("Torn-page protection. OFF trades crash safety for write throughput.").Default(myOn()),
		// doc: .../innodb-parameters.html#sysvar_innodb_file_per_table
		myOnOff("innodb_file_per_table").Title("File per table").Group("InnoDB").
			Desc("Store each table in its own .ibd tablespace.").Default(myOn()),
		// doc: .../innodb-parameters.html#sysvar_innodb_dedicated_server
		myOnOff("innodb_dedicated_server").Title("Dedicated server").Group("InnoDB").
			Desc("Let InnoDB size the buffer pool and redo log from the machine's RAM, ignoring the values above.").
			Default(myOff()),

		// ---- Binlog / GTID ------------------------------------------------
		// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#option_mysqld_log-bin
		schemapb.Str("log_bin").Title("Binary log base name").Group("Binlog").
			Desc("Base name of the binary log files; binary logging is on by default since 8.0.").
			MinLen(1).MaxLen(200).Default("binlog"),
		// doc: .../replication-options-binary-log.html#sysvar_binlog_format
		schemapb.Choice("binlog_format").Title("Binlog format").Group("Binlog").
			Desc("Row-based logging is the default and the only format group replication accepts.").
			Opt(schemapb.StrV("ROW"), "ROW").
			Opt(schemapb.StrV("STATEMENT"), "STATEMENT (deprecated)").
			Opt(schemapb.StrV("MIXED"), "MIXED (deprecated)").
			Default(schemapb.StrV("ROW")),
		// doc: .../replication-options-binary-log.html#sysvar_binlog_row_image
		schemapb.Choice("binlog_row_image").Title("Binlog row image").Group("Binlog").
			Desc("How much of each row goes into the binary log.").
			Opt(schemapb.StrV("full"), "full").
			Opt(schemapb.StrV("minimal"), "minimal").
			Opt(schemapb.StrV("noblob"), "noblob").
			Default(schemapb.StrV("full")),
		// doc: .../replication-options-binary-log.html#sysvar_sync_binlog
		schemapb.Int64("sync_binlog").Title("Sync binlog").Group("Binlog").
			Desc("fsync the binary log every N commit groups; 1 is the durable default, 0 leaves it to the OS.").
			Gte(0).Lte(4294967295).Default(1),
		// doc: .../replication-options-binary-log.html#sysvar_binlog_expire_logs_seconds
		schemapb.Int64("binlog_expire_logs_seconds").Title("Binlog retention").Group("Binlog").
			Desc("Binary log purge age; 0 disables automatic purging. Upstream default 30 days.").
			Unit("s").Gte(0).Lte(4294967295).Default(2592000),
		// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-gtids.html#sysvar_gtid_mode
		schemapb.Choice("gtid_mode").Title("GTID mode").Group("Binlog").
			Desc("GTID-based replication. Group replication requires ON.").
			Opt(schemapb.StrV("OFF"), "OFF").
			Opt(schemapb.StrV("OFF_PERMISSIVE"), "OFF_PERMISSIVE").
			Opt(schemapb.StrV("ON_PERMISSIVE"), "ON_PERMISSIVE").
			Opt(schemapb.StrV("ON"), "ON").
			Default(schemapb.StrV("OFF")),
		// doc: .../replication-options-gtids.html#sysvar_enforce_gtid_consistency
		schemapb.Choice("enforce_gtid_consistency").Title("Enforce GTID consistency").Group("Binlog").
			Desc("Reject statements that cannot be logged transactionally; ON is required with gtid_mode=ON.").
			Opt(schemapb.StrV("OFF"), "OFF").
			Opt(schemapb.StrV("WARN"), "WARN").
			Opt(schemapb.StrV("ON"), "ON").
			Default(schemapb.StrV("OFF")),
		// doc: .../replication-options-binary-log.html#sysvar_log_replica_updates (8.0.26+; the only name in 8.4)
		myOnOff("log_replica_updates").Title("Log replica updates").Group("Binlog").
			Desc("Write replicated changes to this server's own binary log — required for chained replication and for group replication.").
			Default(myOn()),

		// ---- Replication ---------------------------------------------------
		// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-source.html#sysvar_rpl_semi_sync_source_enabled
		myOnOff("rpl_semi_sync_source_enabled").Title("Semisync source").Group("Replication").
			Desc("Semisynchronous source side; loads semisync_source.so via plugin_load_add.").Default(myOff()),
		// doc: .../replication-options-source.html#sysvar_rpl_semi_sync_source_timeout
		schemapb.Int64("rpl_semi_sync_source_timeout").Title("Semisync timeout").Group("Replication").
			Desc("How long the source waits for a replica ack before falling back to async.").
			Unit("ms").Gte(0).Lte(4294967295).Default(10000),
		// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-replica.html#sysvar_rpl_semi_sync_replica_enabled
		myOnOff("rpl_semi_sync_replica_enabled").Title("Semisync replica").Group("Replication").
			Desc("Semisynchronous replica side; loads semisync_replica.so via plugin_load_add.").Default(myOff()),
		// doc: .../replication-options-replica.html#sysvar_replica_parallel_workers
		schemapb.Int64("replica_parallel_workers").Title("Applier workers").Group("Replication").
			Desc("Parallel applier threads on the replica; 0 = single-threaded.").
			Gte(0).Lte(1024).Default(4),
		// doc: .../replication-options-replica.html#sysvar_replica_preserve_commit_order
		myOnOff("replica_preserve_commit_order").Title("Preserve commit order").Group("Replication").
			Desc("Commit in source order when applying in parallel.").Default(myOn()),
		// doc: https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_read_only
		myOnOff("read_only").Title("Read only").Group("Replication").
			Desc("Reject writes from clients without CONNECTION_ADMIN — set on replicas.").Default(myOff()),
		// doc: .../server-system-variables.html#sysvar_super_read_only
		myOnOff("super_read_only").Title("Super read only").Group("Replication").
			Desc("Reject writes from every account, administrators included; implies read_only.").Default(myOff()),

		// ---- Group replication --------------------------------------------
		schemapb.Bool("group_replication").Title("Group replication").Group("Group replication").
			Desc("Enable the group replication plugin; renders the whole group_replication_* block.").
			Default(false),
		// doc: https://dev.mysql.com/doc/refman/8.0/en/group-replication-system-variables.html#sysvar_group_replication_group_name
		schemapb.Str("group_replication_group_name").Title("Group name").Group("Group replication").
			Desc("UUID naming the group; identical on every member.").
			Format(schemapb.FormatUUID).Nullable(),
		// doc: .../group-replication-system-variables.html#sysvar_group_replication_start_on_boot
		myOnOff("group_replication_start_on_boot").Title("Start on boot").Group("Group replication").
			Desc("Join the group automatically at server start.").Default(myOff()),
		// doc: .../group-replication-system-variables.html#sysvar_group_replication_bootstrap_group
		myOnOff("group_replication_bootstrap_group").Title("Bootstrap group").Group("Group replication").
			Desc("Create the group instead of joining it — exactly one member, exactly once.").Default(myOff()),
		// doc: .../group-replication-system-variables.html#sysvar_group_replication_single_primary_mode
		myOnOff("group_replication_single_primary_mode").Title("Single primary mode").Group("Group replication").
			Desc("One writable primary (ON) or multi-primary (OFF).").Default(myOn()),
		// doc: .../group-replication-system-variables.html#sysvar_group_replication_enforce_update_everywhere_checks
		myOnOff("group_replication_enforce_update_everywhere_checks").Title("Update-everywhere checks").Group("Group replication").
			Desc("Multi-primary safety checks; must be OFF in single-primary mode.").Default(myOff()),
		// doc: https://dev.mysql.com/doc/refman/8.4/en/group-replication-system-variables.html#sysvar_group_replication_consistency
		schemapb.Choice("group_replication_consistency").Title("Transaction consistency").Group("Group replication").
			Desc("Group synchronization before or after transactions. BEFORE waits for preceding transactions before reads, allowing read-after-write through replicas; this is separate from SQL transaction isolation.").
			Opt(schemapb.StrV("EVENTUAL"), "Eventual").Opt(schemapb.StrV("BEFORE_ON_PRIMARY_FAILOVER"), "Before primary failover").
			Opt(schemapb.StrV("BEFORE"), "Before transaction").Opt(schemapb.StrV("AFTER"), "After transaction").
			Opt(schemapb.StrV("BEFORE_AND_AFTER"), "Before and after").Nullable(),

		// ---- Connections ---------------------------------------------------
		// doc: https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_bind_address
		schemapb.Str("bind_address").Title("Bind address").Group("Connections").
			Desc("Addresses mysqld listens on; * or 0.0.0.0 for all IPv4.").
			MinLen(1).MaxLen(255).Default("0.0.0.0"),
		// doc: .../server-system-variables.html#sysvar_port
		schemapb.Int64("port").Title("Port").Group("Connections").
			Desc("TCP port for the classic MySQL protocol.").Gte(1).Lte(65535).Default(3306),
		// doc: .../server-system-variables.html#sysvar_max_connections
		schemapb.Int64("max_connections").Title("Max connections").Group("Connections").
			Desc("Maximum simultaneous client connections.").Gte(1).Lte(100000).Default(151),
		// doc: .../server-system-variables.html#sysvar_max_connect_errors
		schemapb.Int64("max_connect_errors").Title("Max connect errors").Group("Connections").
			Desc("Interrupted handshakes from one host before it is blocked.").
			Gte(1).Lte(4294967295).Default(100),
		// doc: .../server-system-variables.html#sysvar_thread_cache_size
		schemapb.Int64("thread_cache_size").Title("Thread cache size").Group("Connections").
			Desc("Threads kept for reuse after a client disconnects.").Gte(0).Lte(16384).Default(9),
		// doc: .../server-system-variables.html#sysvar_wait_timeout
		schemapb.Int64("wait_timeout").Title("Wait timeout").Group("Connections").
			Desc("Idle seconds before a non-interactive connection is closed.").
			Unit("s").Gte(1).Lte(31536000).Default(28800),
		// doc: .../server-system-variables.html#sysvar_interactive_timeout
		schemapb.Int64("interactive_timeout").Title("Interactive timeout").Group("Connections").
			Desc("Idle seconds before an interactive connection is closed.").
			Unit("s").Gte(1).Lte(31536000).Default(28800),
		// doc: .../server-system-variables.html#sysvar_max_allowed_packet
		schemapb.Int64("max_allowed_packet").Title("Max allowed packet").Group("Connections").
			Desc("Largest single packet or generated string.").
			Unit("MB").Gte(1).Lte(1024).Default(64),

		// ---- Tables / cache --------------------------------------------------
		// doc: .../server-system-variables.html#sysvar_table_open_cache
		schemapb.Int64("table_open_cache").Title("Table open cache").Group("Tables").
			Desc("Open table handles cached across all sessions.").Gte(1).Lte(1048576).Default(4000),
		// doc: .../server-system-variables.html#sysvar_table_definition_cache
		schemapb.Int64("table_definition_cache").Title("Table definition cache").Group("Tables").
			Desc("Cached table definitions.").Gte(400).Lte(524288).Default(2000),
		// doc: .../server-system-variables.html#sysvar_tmp_table_size
		schemapb.Int64("tmp_table_size").Title("Temp table size").Group("Tables").
			Desc("Largest in-memory internal temporary table before it spills to disk.").
			Unit("MB").Gte(1).Lte(65536).Default(16),
		// doc: .../server-system-variables.html#sysvar_max_heap_table_size
		schemapb.Int64("max_heap_table_size").Title("Max heap table size").Group("Tables").
			Desc("Largest user-created MEMORY table; caps tmp_table_size too.").
			Unit("MB").Gte(1).Lte(65536).Default(16),
		// doc: .../server-system-variables.html#sysvar_open_files_limit
		schemapb.Int64("open_files_limit").Title("Open files limit").Group("Tables").
			Desc("File descriptors mysqld asks the OS for.").Gte(1024).Lte(1048576).Default(5000),

		// ---- SQL --------------------------------------------------------------
		// doc: .../server-system-variables.html#sysvar_sql_mode
		schemapb.Str("sql_mode").Title("SQL mode").Group("SQL").
			Desc("Comma-separated SQL modes; the value below is the 8.0/8.4 upstream default.").
			MaxLen(1024).
			Default("ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"),
		// doc: .../server-system-variables.html#sysvar_character_set_server
		schemapb.Choice("character_set_server").Title("Server character set").Group("SQL").
			Desc("Default character set of the server.").
			Opt(schemapb.StrV("utf8mb4"), "utf8mb4").
			Opt(schemapb.StrV("utf8mb3"), "utf8mb3").
			Opt(schemapb.StrV("latin1"), "latin1").
			Default(schemapb.StrV("utf8mb4")),
		// doc: .../server-system-variables.html#sysvar_collation_server
		schemapb.Str("collation_server").Title("Server collation").Group("SQL").
			Desc("Default collation; must belong to character_set_server.").
			MinLen(1).MaxLen(64).Default("utf8mb4_0900_ai_ci"),
		// doc: .../server-system-variables.html#sysvar_time_zone (option: default_time_zone)
		schemapb.Str("default_time_zone").Title("Default time zone").Group("SQL").
			Desc("Server time zone; a fixed offset keeps benchmark timestamps reproducible.").
			MinLen(1).MaxLen(64).Default("+00:00"),
		// doc: .../server-system-variables.html#sysvar_lower_case_table_names
		schemapb.Int64("lower_case_table_names").Title("Lower case table names").Group("SQL").
			Desc("0 = case-sensitive names as stored, 1 = lowercased, 2 = stored as given, compared lowercase. Fixed at initialization.").
			In(0, 1, 2).Default(0),
		// doc: .../server-system-variables.html#sysvar_transaction_isolation
		schemapb.Choice("transaction_isolation").Title("Transaction isolation").Group("SQL").
			Desc("Default isolation level for new sessions.").
			Opt(schemapb.StrV("READ-UNCOMMITTED"), "READ UNCOMMITTED").
			Opt(schemapb.StrV("READ-COMMITTED"), "READ COMMITTED").
			Opt(schemapb.StrV("REPEATABLE-READ"), "REPEATABLE READ").
			Opt(schemapb.StrV("SERIALIZABLE"), "SERIALIZABLE").
			Default(schemapb.StrV("REPEATABLE-READ")),

		// ---- Performance schema -----------------------------------------------
		// doc: https://dev.mysql.com/doc/refman/8.0/en/performance-schema-system-variables.html#sysvar_performance_schema
		myOnOff("performance_schema").Title("Performance schema").Group("Performance schema").
			Desc("Instrumentation engine; needed by mysqld_exporter's perf_schema collectors.").Default(myOn()),
		// doc: .../performance-schema-system-variables.html#sysvar_performance_schema_max_digest_length
		schemapb.Int64("performance_schema_max_digest_length").Title("Max digest length").Group("Performance schema").
			Desc("Bytes kept per normalized statement digest.").
			Unit("B").Gte(0).Lte(1048576).Default(1024),

		// ---- Logging ------------------------------------------------------------
		// doc: .../server-system-variables.html#sysvar_general_log
		myOnOff("general_log").Title("General log").Group("Logging").
			Desc("Log every statement — very expensive, off during load tests.").Default(myOff()),
		// doc: .../server-system-variables.html#sysvar_slow_query_log
		myOnOff("slow_query_log").Title("Slow query log").Group("Logging").
			Desc("Log statements slower than long_query_time.").Default(myOff()),
		// doc: .../server-system-variables.html#sysvar_long_query_time
		schemapb.Double("long_query_time").Title("Long query time").Group("Logging").
			Desc("Slow-query threshold; fractional seconds are allowed.").
			Unit("s").Gte(0).Lte(31536000).Default(10),
		// doc: .../server-system-variables.html#sysvar_log_slow_extra (8.0.14+)
		myOnOff("log_slow_extra").Title("Slow log extra fields").Group("Logging").
			Desc("Add per-statement counters to slow-log records.").Default(myOff()),
		// doc: .../server-system-variables.html#sysvar_log_error_verbosity
		schemapb.Int64("log_error_verbosity").Title("Error log verbosity").Group("Logging").
			Desc("1 = errors, 2 = errors and warnings, 3 = adds notes.").In(1, 2, 3).Default(2),
	}

	// ---- version-gated fields ----------------------------------------------
	if is80 {
		fields = append(fields,
			// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-replica.html#sysvar_replica_parallel_type
			// (deprecated in 8.0.29, removed in 8.4 — 8.0 only)
			schemapb.Choice("replica_parallel_type").Title("Applier parallel type").Group("Replication").
				Desc("How the applier partitions work. Removed in 8.4, where LOGICAL_CLOCK is always used.").
				Opt(schemapb.StrV("LOGICAL_CLOCK"), "LOGICAL_CLOCK").
				Opt(schemapb.StrV("DATABASE"), "DATABASE").
				Default(schemapb.StrV("LOGICAL_CLOCK")),
			// doc: https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_transaction_write_set_extraction
			// (removed in 8.4)
			schemapb.Choice("transaction_write_set_extraction").Title("Write set extraction").Group("Group replication").
				Desc("Write-set hashing algorithm; XXHASH64 is required by group replication. Removed in 8.4, where write sets are always extracted.").
				Opt(schemapb.StrV("XXHASH64"), "XXHASH64").
				Opt(schemapb.StrV("MURMUR32"), "MURMUR32").
				Opt(schemapb.StrV("OFF"), "OFF").
				Default(schemapb.StrV("XXHASH64")),
			// doc: .../replication-options-binary-log.html#sysvar_binlog_transaction_dependency_tracking (removed in 8.4)
			schemapb.Choice("binlog_transaction_dependency_tracking").Title("Dependency tracking").Group("Group replication").
				Desc("Source of the parallelization info written into the binary log. Removed in 8.4, where WRITESET is always used.").
				Opt(schemapb.StrV("COMMIT_ORDER"), "COMMIT_ORDER").
				Opt(schemapb.StrV("WRITESET"), "WRITESET").
				Opt(schemapb.StrV("WRITESET_SESSION"), "WRITESET_SESSION").
				Default(schemapb.StrV("WRITESET")),
			// doc: https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_default_authentication_plugin
			// (deprecated in 8.0.27, removed in 8.4 in favor of authentication_policy)
			schemapb.Choice("default_authentication_plugin").Title("Default auth plugin").Group("SQL").
				Desc("Authentication plugin for new accounts. Removed in 8.4 — use authentication_policy there.").
				Opt(schemapb.StrV("caching_sha2_password"), "caching_sha2_password").
				Opt(schemapb.StrV("mysql_native_password"), "mysql_native_password").
				Opt(schemapb.StrV("sha256_password"), "sha256_password").
				Default(schemapb.StrV("caching_sha2_password")),
		)
	} else {
		fields = append(fields,
			// doc: https://dev.mysql.com/doc/refman/8.4/en/server-system-variables.html#sysvar_authentication_policy
			schemapb.Str("authentication_policy").Title("Authentication policy").Group("SQL").
				Desc("Factor list for CREATE USER; '*,,' means factor 1 uses default_authentication_plugin's successor (caching_sha2_password) and factors 2/3 are unused. Replaces default_authentication_plugin.").
				MinLen(1).MaxLen(255).Default("*,,"),
			// doc: https://dev.mysql.com/doc/refman/8.4/en/native-pluggable-authentication.html
			// (mysql_native_password is disabled by default in 8.4)
			myOnOff("mysql_native_password").Title("mysql_native_password").Group("SQL").
				Desc("The deprecated native plugin is off by default in 8.4; ON only for legacy clients.").
				Default(myOff()),
		)
	}

	// ---- cluster-filled ------------------------------------------------------
	fields = append(fields,
		schemapb.Int64("server_id").Title("Server id").Group("Cluster").
			Desc("Unique replication id; filled by the server from topology.").
			Gte(1).Lte(4294967295).Nullable(),
		schemapb.Str("report_host").Title("Report host").Group("Cluster").
			Desc("Address this member reports to the source/group; filled by the server from topology.").
			MaxLen(255).Nullable(),
		schemapb.Str("group_replication_local_address").Title("GR local address").Group("Cluster").
			Desc("host:port of this member's group communication endpoint; filled by the server from topology.").
			MaxLen(255).Nullable(),
		schemapb.Str("group_replication_group_seeds").Title("GR group seeds").Group("Cluster").
			Desc("Comma-separated seed endpoints of the group; filled by the server from topology.").
			MaxLen(4096).Nullable(),
	)

	// ---- custom + computed blocks -------------------------------------------
	grLines := myJoin(
		`"plugin_load_add = group_replication.so\n"`,
		myEmit("loose-group_replication_group_name", "group_replication_group_name"),
		myEmit("loose-group_replication_start_on_boot", "group_replication_start_on_boot"),
		myEmit("loose-group_replication_bootstrap_group", "group_replication_bootstrap_group"),
		myEmit("loose-group_replication_single_primary_mode", "group_replication_single_primary_mode"),
		myEmit("loose-group_replication_enforce_update_everywhere_checks", "group_replication_enforce_update_everywhere_checks"),
		myEmit("loose-group_replication_local_address", "group_replication_local_address"),
		myEmit("loose-group_replication_group_seeds", "group_replication_group_seeds"),
		myEmit("loose-group_replication_consistency", "group_replication_consistency"),
	)
	if is80 {
		grLines = myJoin(grLines,
			myEmit("transaction_write_set_extraction", "transaction_write_set_extraction"),
			myEmit("binlog_transaction_dependency_tracking", "binlog_transaction_dependency_tracking"),
		)
	}

	semisync := myJoin(
		// mysqld --initialize ignores plugin_load_add; loose options allow the
		// initial datadir creation. The recipe checks the plugin after startup.
		`((("rpl_semi_sync_source_enabled" in root) && root.rpl_semi_sync_source_enabled == "ON") ?
			"plugin_load_add = semisync_source.so\nloose-rpl_semi_sync_source_enabled = ON\n" +`+
			myEmit("loose-rpl_semi_sync_source_timeout", "rpl_semi_sync_source_timeout")+` : "")`,
		`((("rpl_semi_sync_replica_enabled" in root) && root.rpl_semi_sync_replica_enabled == "ON") ?
			"plugin_load_add = semisync_replica.so\nloose-rpl_semi_sync_replica_enabled = ON\n" : "")`,
	)

	fields = append(fields,
		customField().MaxEntries(128),

		schemapb.Computed("cluster_lines", myJoin(
			myEmit("server_id", "server_id"),
			myEmit("report_host", "report_host"),
		)).Result(schemapb.ResultString).Group("Rendered").
			Title("Cluster lines").Desc("Rendered topology lines (server_id, report_host)."),

		schemapb.Computed("semisync_lines", semisync).
			Result(schemapb.ResultString).Group("Rendered").
			Title("Semisync lines").Desc("plugin_load_add and rpl_semi_sync_* lines, emitted only when semisync is on."),

		schemapb.Computed("group_replication_lines",
			`((("group_replication" in root) && root.group_replication) ? (`+grLines+`) : "")`).
			Result(schemapb.ResultString).Group("Rendered").
			Title("Group replication lines").Desc("The whole group_replication_* block, emitted only when group replication is on."),

		customRendered(),
	)

	tmpl := myTemplate(major, is80)

	b := schemapb.NewSchema(myID(major)).
		Descr("MySQL "+major+" server configuration rendered to /etc/mysql/my.cnf ([mysqld] section).").
		Strict().Coerce().
		Fields(fields...).
		Rules(
			schemapb.Rule(`!("innodb_io_capacity" in root) || !("innodb_io_capacity_max" in root) || int(root.innodb_io_capacity_max) >= int(root.innodb_io_capacity)`,
				"innodb_io_capacity_max must be >= innodb_io_capacity").ID("io-capacity-order"),
			schemapb.Rule(myGR(`("gtid_mode" in root) && root.gtid_mode == "ON"`),
				"group replication requires gtid_mode = ON").ID("gr-gtid-mode"),
			schemapb.Rule(myGR(`("enforce_gtid_consistency" in root) && root.enforce_gtid_consistency == "ON"`),
				"group replication requires enforce_gtid_consistency = ON").ID("gr-gtid-consistency"),
			schemapb.Rule(myGR(`!("binlog_format" in root) || root.binlog_format == "ROW"`),
				"group replication requires binlog_format = ROW").ID("gr-binlog-format"),
			schemapb.Rule(myGR(`!("log_replica_updates" in root) || root.log_replica_updates == "ON"`),
				"group replication requires log_replica_updates = ON").ID("gr-log-replica-updates"),
			schemapb.Rule(`!("group_replication_enforce_update_everywhere_checks" in root) || root.group_replication_enforce_update_everywhere_checks == "OFF" || (("group_replication_single_primary_mode" in root) && root.group_replication_single_primary_mode == "OFF")`,
				"enforce_update_everywhere_checks must be OFF in single-primary mode").ID("gr-single-primary"),
			schemapb.Rule(`!("super_read_only" in root) || root.super_read_only == "OFF" || (("read_only" in root) && root.read_only == "ON")`,
				"super_read_only = ON implies read_only = ON").ID("read-only-implies"),
			schemapb.Rule(`!("max_heap_table_size" in root) || !("tmp_table_size" in root) || int(root.max_heap_table_size) >= int(root.tmp_table_size)`,
				"max_heap_table_size must be >= tmp_table_size").ID("heap-ge-tmp"),
			customKeyRule(),
		).
		RequiredWhen("group_replication_group_name", `("group_replication" in root) && root.group_replication`).
		Template("conf", tmpl)

	return b.MustBuild()
}

// myMajorID maps the my.cnf major string to the numeric schema major:
// 8.0 -> 80, 8.4 -> 84 (ids.ID takes a single integer version axis).
func myID(major string) *schemapb.SchemaIdentity {
	if major == "8.0" {
		return ids.CfgMinor("my.cnf", 8, 0)
	}

	return ids.CfgMinor("my.cnf", 8, 4)
}

// myTemplate builds the [mysqld] file for one major.
func myTemplate(major string, is80 bool) string {
	var b strings.Builder

	b.WriteString("# MySQL " + major + " — generated by stroppy, do not edit by hand\n[mysqld]\n\n")

	b.WriteString(`# InnoDB
innodb_buffer_pool_size = {{{values.innodb_buffer_pool_size}}}M
innodb_buffer_pool_instances = {{{values.innodb_buffer_pool_instances}}}
innodb_redo_log_capacity = {{{values.innodb_redo_log_capacity}}}M
innodb_flush_log_at_trx_commit = {{{values.innodb_flush_log_at_trx_commit}}}
innodb_flush_method = {{{values.innodb_flush_method}}}
innodb_io_capacity = {{{values.innodb_io_capacity}}}
innodb_io_capacity_max = {{{values.innodb_io_capacity_max}}}
innodb_read_io_threads = {{{values.innodb_read_io_threads}}}
innodb_write_io_threads = {{{values.innodb_write_io_threads}}}
innodb_thread_concurrency = {{{values.innodb_thread_concurrency}}}
innodb_lock_wait_timeout = {{{values.innodb_lock_wait_timeout}}}
innodb_doublewrite = {{{values.innodb_doublewrite}}}
innodb_file_per_table = {{{values.innodb_file_per_table}}}
innodb_dedicated_server = {{{values.innodb_dedicated_server}}}

# Binary log and GTID
log_bin = {{{values.log_bin}}}
binlog_format = {{{values.binlog_format}}}
binlog_row_image = {{{values.binlog_row_image}}}
sync_binlog = {{{values.sync_binlog}}}
binlog_expire_logs_seconds = {{{values.binlog_expire_logs_seconds}}}
gtid_mode = {{{values.gtid_mode}}}
enforce_gtid_consistency = {{{values.enforce_gtid_consistency}}}
log_replica_updates = {{{values.log_replica_updates}}}
{{{values.cluster_lines}}}
# Replication
replica_parallel_workers = {{{values.replica_parallel_workers}}}
replica_preserve_commit_order = {{{values.replica_preserve_commit_order}}}
`)

	if is80 {
		b.WriteString("replica_parallel_type = {{{values.replica_parallel_type}}}\n")
	}

	b.WriteString(`read_only = {{{values.read_only}}}
super_read_only = {{{values.super_read_only}}}
{{{values.semisync_lines}}}
{{{values.group_replication_lines}}}
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
transaction_isolation = {{{values.transaction_isolation}}}
`)

	if is80 {
		b.WriteString("default_authentication_plugin = {{{values.default_authentication_plugin}}}\n")
	} else {
		b.WriteString("authentication_policy = {{{values.authentication_policy}}}\nmysql_native_password = {{{values.mysql_native_password}}}\n")
	}

	b.WriteString(`
# Performance schema
performance_schema = {{{values.performance_schema}}}
performance_schema_max_digest_length = {{{values.performance_schema_max_digest_length}}}

# Logging
general_log = {{{values.general_log}}}
slow_query_log = {{{values.slow_query_log}}}
long_query_time = {{{values.long_query_time}}}
log_slow_extra = {{{values.log_slow_extra}}}
log_error_verbosity = {{{values.log_error_verbosity}}}

# Custom
{{{values.custom_rendered}}}
`)

	return b.String()
}
