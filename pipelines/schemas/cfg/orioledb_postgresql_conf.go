package cfg

import (
	"fmt"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// OrioledbPostgresqlConf16 is cfg.orioledb.postgresql.conf@16.
func OrioledbPostgresqlConf16() *schemapb.Schema { return orioledbPostgresqlConf(16) }

// OrioledbPostgresqlConf17 is cfg.orioledb.postgresql.conf@17.
func OrioledbPostgresqlConf17() *schemapb.Schema { return orioledbPostgresqlConf(17) }

// OrioledbPostgresqlConf18 is cfg.orioledb.postgresql.conf@18.
func OrioledbPostgresqlConf18() *schemapb.Schema { return orioledbPostgresqlConf(18) }

// orioledbPostgresqlConf builds cfg.orioledb.postgresql.conf@<major>: the
// stock postgresql.conf of that PostgreSQL major plus the orioledb.* GUCs.
//
// OrioleDB ships as a patched PostgreSQL plus a table access method, so the
// whole PostgreSQL parameter surface still applies verbatim — this schema
// composes the @<major> parameter table (pgConfParts, with all its own
// version gates: effective_io_concurrency, io_method/io_workers,
// autovacuum_worker_slots and the log_connections aspect list on 18,
// reserved_connections on 16+, transaction_timeout on 17+) rather than
// restating it.
//
// The image majors are the ones the orioledb params schema offers, i.e. the
// docker tags *-pg16 / *-pg17 / *-pg18 of orioledb/postgres.
// doc: https://hub.docker.com/r/orioledb/postgres
//
// Every orioledb.* default below is from the OrioleDB configuration guide,
// https://www.orioledb.com/docs/usage/configuration (see the `doc:` markers).
// The orioledb.* surface itself is NOT version-gated: the extension builds the
// same GUC table on every supported PostgreSQL major.
func orioledbPostgresqlConf(major uint64) *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("orioledb.postgresql.conf", major)).
		Descr(fmt.Sprintf(
			"postgresql.conf for the OrioleDB build of PostgreSQL %d: the stock %d parameters plus the orioledb.* settings.",
			major, major)).
		Strict().Coerce().
		Fields(pgConfFields(major, orioledbExtra)...).
		Rules(append(pgConfRules(),
			schemapb.Rule(`("extensions" in root) && ("orioledb" in root.extensions)`,
				"shared_preload_libraries must include orioledb").ID("orioledb-preloaded"),
			// beta17 reserves the pg_stat_statements prefix while loading OrioleDB.
			// Load pg_stat_statements first so its configured GUCs are not discarded.
			// doc: https://github.com/orioledb/orioledb/blob/main/src/orioledb.c
			schemapb.Rule(`!("extensions" in root) || !("pg_stat_statements" in root.extensions) || root.extensions.filter(x, x == "orioledb" || x == "pg_stat_statements")[0] == "pg_stat_statements"`,
				"pg_stat_statements must precede orioledb in preloaded libraries").ID("orioledb-statements-order"),
		)...).
		Template("conf", pgConfTemplate(major, orioledbExtra)).
		MustBuild()
}

// orioledbExtra appends the orioledb.* block to the PostgreSQL file.
//
// Field names are identifiers (schemapb requirement), so every orioledb GUC is
// declared as orioledb_<name> and rendered as `orioledb.<name>`.
//
//nolint:funlen // one flat parameter table plus its template
func orioledbExtra(add func(schemapb.FieldDef), l *pgLines) {
	l.section("OrioleDB")

	// doc: https://www.orioledb.com/docs/usage/configuration
	line := func(name string, unit string) {
		l.f("orioledb.%s = {{{values.orioledb_%s}}}%s", name, name, unit)
	}

	add(schemapb.Int64("orioledb_main_buffers").Title("orioledb.main_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.main_buffers, default 64MB
		Desc("Shared memory OrioleDB uses to cache hot pages; the OrioleDB counterpart of shared_buffers.").
		Gte(1).Lte(4194304).Default(64))
	line("main_buffers", "MB")

	add(schemapb.Int64("orioledb_undo_buffers").Title("orioledb.undo_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.undo_buffers; beta17 src/orioledb.c _PG_init
		Desc("Ring buffer holding older row and page versions. Leave unset for the engine default, which grows with the maximum process count; an explicit value must meet the engine's process-dependent minimum.").
		Gte(1).Lte(1048576).Nullable())
	l.f("{{#values.orioledb_undo_buffers}}orioledb.undo_buffers = {{{.}}}MB\n{{/values.orioledb_undo_buffers}}")

	add(schemapb.Int64("orioledb_free_tree_buffers").Title("orioledb.free_tree_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.free_tree_buffers, default 8MB
		Desc("Shared memory for the free-space metadata of compressed tables.").
		Gte(1).Lte(1048576).Default(8))
	line("free_tree_buffers", "MB")

	add(schemapb.Int64("orioledb_catalog_buffers").Title("orioledb.catalog_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.catalog_buffers, default 8MB
		Desc("Shared memory for table metadata.").
		Gte(1).Lte(1048576).Default(8))
	line("catalog_buffers", "MB")

	add(schemapb.Int64("orioledb_temp_buffers").Title("orioledb.temp_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.temp_buffers, default 64MB
		Desc("Per-backend buffer pool for OrioleDB temporary tables.").
		Gte(1).Lte(1048576).Default(64))
	line("temp_buffers", "MB")

	add(schemapb.Int64("orioledb_xid_buffers").Title("orioledb.xid_buffers").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.xid_buffers, default 1MB
		Desc("In-memory buffer for OrioleDB transaction ids.").
		Gte(1).Lte(1048576).Default(1))
	line("xid_buffers", "MB")

	add(schemapb.Double("orioledb_checkpoint_completion_ratio").Title("orioledb.checkpoint_completion_ratio").
		Group("OrioleDB").
		// doc: configuration — orioledb.checkpoint_completion_ratio, default 0.5
		Desc("Share of the checkpoint window spent on OrioleDB tables; the remainder goes to heap tables.").
		Gte(0).Lte(1).Default(0.5))
	line("checkpoint_completion_ratio", "")

	add(schemapb.Int64("orioledb_max_io_concurrency").Title("orioledb.max_io_concurrency").Group("OrioleDB").
		// doc: configuration — orioledb.max_io_concurrency, default 0 (derived)
		Desc("Cap on concurrent OrioleDB I/O operations; 0 lets OrioleDB derive it from main_buffers.").
		Gte(0).Lte(4096).Default(0))
	line("max_io_concurrency", "")

	add(schemapb.Int64("orioledb_bgwriter_num_workers").Title("orioledb.bgwriter_num_workers").Group("OrioleDB").
		// doc: configuration — orioledb.bgwriter_num_workers, default 1
		Desc("Background writer processes flushing OrioleDB pages.").
		Gte(0).Lte(64).Default(1))
	line("bgwriter_num_workers", "")

	add(schemapb.Int64("orioledb_recovery_pool_size").Title("orioledb.recovery_pool_size").Group("OrioleDB").
		// doc: configuration — orioledb.recovery_pool_size, default 3
		Desc("Worker processes replaying OrioleDB WAL during recovery.").
		Gte(1).Lte(256).Default(3))
	line("recovery_pool_size", "")

	add(schemapb.Int64("orioledb_recovery_idx_pool_size").Title("orioledb.recovery_idx_pool_size").Group("OrioleDB").
		// doc: configuration — orioledb.recovery_idx_pool_size, default 3
		Desc("Workers building indexes in parallel during recovery.").
		Gte(1).Lte(256).Default(3))
	line("recovery_idx_pool_size", "")

	add(schemapb.Int64("orioledb_recovery_queue_size").Title("orioledb.recovery_queue_size").Group("OrioleDB").Unit("MB").
		// doc: configuration — orioledb.recovery_queue_size, default 8MB
		Desc("Shared memory for the message queues between recovery workers.").
		Gte(1).Lte(1048576).Default(8))
	line("recovery_queue_size", "MB")

	add(schemapb.Int64("orioledb_default_compress").Title("orioledb.default_compress").Group("OrioleDB").
		// doc: configuration — orioledb.default_compress, default -1 (off)
		Desc("Default compression level for new OrioleDB tables; -1 disables compression.").
		Gte(-1).Lte(22).Default(-1))
	line("default_compress", "")

	add(schemapb.Choice("orioledb_serializable").Title("orioledb.serializable").Group("OrioleDB").
		// doc: src/orioledb.c serializable_mode_options — table_lock (default),
		// error, repeatable_read. There is no `ignore`.
		Desc("How OrioleDB answers a request for SERIALIZABLE isolation: table_lock takes a coarse ExclusiveLock per relation, error rejects the transaction, repeatable_read silently downgrades it.").
		Opt(schemapb.StrV("table_lock"), "table_lock").
		Opt(schemapb.StrV("error"), "error").
		Opt(schemapb.StrV("repeatable_read"), "repeatable_read").
		Default(schemapb.StrV("table_lock")))
	line("serializable", "")

	add(onOff("orioledb_use_mmap", "off").Title("orioledb.use_mmap").Group("OrioleDB").
		// doc: configuration — orioledb.use_mmap, default off
		Desc("Access the raw block device through mmap instead of read/write; only with device_filename."))
	line("use_mmap", "")

	add(schemapb.Int64("orioledb_device_length").Title("orioledb.device_length").Group("OrioleDB").Unit("MB").
		// doc: src/orioledb.c "orioledb.device_length" — default 0, minimum 0
		Desc("Usable length of the raw block device; only meaningful with device_filename, and 0 (the upstream default) means no device.").
		Gte(0).Lte(1073741824).Default(0))
	line("device_length", "MB")

	add(onOff("orioledb_enable_rewind", "off").Title("orioledb.enable_rewind").Group("OrioleDB").
		// doc: configuration — orioledb.enable_rewind, default off (experimental)
		Desc("Experimental undo-based rewind: keeps committed transactions rewindable."))
	line("enable_rewind", "")

	add(onOff("orioledb_enable_stopevents", "off").Title("orioledb.enable_stopevents").Group("OrioleDB").
		// doc: configuration — orioledb.enable_stopevents, default off
		Desc("Debug-only stop events used by the OrioleDB test suite; costs throughput."))
	line("enable_stopevents", "")

	add(onOff("orioledb_debug_disable_bgwriter", "off").Title("orioledb.debug_disable_bgwriter").Group("OrioleDB").
		// doc: configuration — orioledb.debug_disable_bgwriter, default off
		Desc("Debug-only: never start the OrioleDB background writer."))
	line("debug_disable_bgwriter", "")

	add(schemapb.Str("orioledb_device_filename").Title("orioledb.device_filename").Group("OrioleDB").
		// doc: configuration — orioledb.device_filename, unset by default
		Desc("Raw block device OrioleDB stores data on instead of the filesystem. Unset means normal files.").
		MinLen(1).MaxLen(512).Nullable())
	l.f("{{#values.orioledb_device_filename}}orioledb.device_filename = '{{{.}}}'\n{{/values.orioledb_device_filename}}")
}
