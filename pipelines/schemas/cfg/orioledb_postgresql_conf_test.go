package cfg

import (
	"testing"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

// orioledbDefaultLines are the orioledb.* defaults, identical on every
// PostgreSQL major: the extension registers the same GUC table everywhere.
var orioledbDefaultLines = []string{
	"orioledb.main_buffers = 64MB",
	"orioledb.free_tree_buffers = 8MB",
	"orioledb.catalog_buffers = 8MB",
	"orioledb.temp_buffers = 64MB",
	"orioledb.xid_buffers = 1MB",
	"orioledb.checkpoint_completion_ratio = 0.5",
	"orioledb.max_io_concurrency = 0",
	"orioledb.bgwriter_num_workers = 1",
	"orioledb.recovery_pool_size = 3",
	"orioledb.recovery_idx_pool_size = 3",
	"orioledb.recovery_queue_size = 8MB",
	"orioledb.default_compress = -1",
	"orioledb.serializable = table_lock",
	"orioledb.use_mmap = off",
	"orioledb.device_length = 0MB",
	"orioledb.enable_rewind = off",
	"orioledb.enable_stopevents = off",
	"orioledb.debug_disable_bgwriter = off",
}

// runOrioledb is the shared body of the three per-major tests: the stock
// postgresql.conf cases of that major, plus the orioledb.* block.
func runOrioledb(t *testing.T, s *schemapb.Schema, extra map[string]any) string {
	t.Helper()

	minimal := map[string]any{"extensions": []any{"orioledb"}}

	full := pgFull()
	for k, v := range extra {
		full[k] = v
	}

	full["extensions"] = []any{"pg_stat_statements", "orioledb"}
	full["orioledb_main_buffers"] = int64(8192)
	full["orioledb_undo_buffers"] = int64(256)
	full["orioledb_checkpoint_completion_ratio"] = 0.7
	full["orioledb_max_io_concurrency"] = int64(512)
	full["orioledb_bgwriter_num_workers"] = int64(4)
	full["orioledb_serializable"] = "error"
	full["orioledb_enable_stopevents"] = "off"
	full["orioledb_device_filename"] = "/dev/nvme0n1"

	schematest.Run(t, s, schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: append(pgInvalidWithOrioledb(), schematest.Invalid{
			Value: map[string]any{"extensions": []any{"pg_stat_statements"}},
			Code:  "RULE_VIOLATED", Path: "orioledb-preloaded",
		}),
		Render: "conf",
		Contains: []string{
			"shared_preload_libraries = '",
			"# --- OrioleDB ---",
			"orioledb.main_buffers = ",
			"orioledb.checkpoint_completion_ratio = ",
			"orioledb.serializable = ",
		},
	})

	// renderDefaults leaves `extensions` unset, so only the orioledb.* block
	// with its documented defaults is asserted here.
	out := renderDefaults(t, s)
	wantLines(t, out, orioledbDefaultLines...)
	dontWantLines(t, out, "orioledb.device_filename", "orioledb.undo_buffers")

	return out
}

func TestOrioledbPostgresqlConf16(t *testing.T) {
	out := runOrioledb(t, OrioledbPostgresqlConf16(), map[string]any{
		"reserved_connections": int64(4),
	})

	// The base is the stock PostgreSQL 16 file: reserved_connections is there,
	// the 17 and 18 parameters are not.
	wantLines(t, out, "shared_buffers = 128MB", "wal_level = replica",
		"effective_io_concurrency = 1", "reserved_connections = 0")
	dontWantLines(t, out, "transaction_timeout", "io_method", "autovacuum_worker_slots")
}

func TestOrioledbPostgresqlConf17(t *testing.T) {
	out := runOrioledb(t, OrioledbPostgresqlConf17(), map[string]any{
		"reserved_connections": int64(4),
		"transaction_timeout":  int64(300000),
	})

	wantLines(t, out, "shared_buffers = 128MB", "wal_level = replica",
		"effective_io_concurrency = 1", "reserved_connections = 0", "transaction_timeout = 0ms")
	dontWantLines(t, out, "io_method", "autovacuum_worker_slots")
}

func TestOrioledbPostgresqlConf18(t *testing.T) {
	out := runOrioledb(t, OrioledbPostgresqlConf18(), map[string]any{
		"reserved_connections":            int64(4),
		"transaction_timeout":             int64(300000),
		"io_method":                       "io_uring",
		"io_workers":                      int64(8),
		"autovacuum_vacuum_max_threshold": int64(50000000),
		"autovacuum_worker_slots":         int64(32),
		"log_connections":                 "receipt,authentication",
	})

	// The 18-only parameters of the stock file come through unchanged.
	wantLines(t, out, "shared_buffers = 128MB", "wal_level = replica",
		"effective_io_concurrency = 16", "io_method = worker",
		"transaction_timeout = 0ms", "autovacuum_worker_slots = 16")
}

// pgInvalidWithOrioledb reuses the postgresql.conf invalid cases, adding the
// orioledb preload so they trip their own rule and not orioledb-preloaded.
func pgInvalidWithOrioledb() []schematest.Invalid {
	out := make([]schematest.Invalid, 0, len(pgInvalid()))

	for _, inv := range pgInvalid() {
		inv.Value["extensions"] = []any{"orioledb"}
		out = append(out, inv)
	}

	return out
}
