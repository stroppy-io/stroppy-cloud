package workload

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func minimalSegment(name string) map[string]any {
	return map[string]any{
		"name":     name,
		"workload": map[string]any{"script": "tpcc/tx"},
		"run":      map[string]any{"executor": "constant-vus", "duration": "60s"},
	}
}

func TestSegment(t *testing.T) {
	full := map[string]any{
		"name": "steady-state",
		"workload": map[string]any{
			"script":          "tpcc/procs",
			"scale_factor":    int64(10),
			"warehouse_start": int64(1),
			"load_items":      true,
			"load_workers":    int64(8),
			"pg_unlogged":     true,
			"pacing":          true,
			"retry_attempts":  int64(5),
			"tx_isolation":    "repeatable_read",
			"sql_dialect":     "ydb_no_indexes",
		},
		"run": map[string]any{
			"executor":      "shared-iterations",
			"vus":           int64(64),
			"iterations":    int64(100000),
			"query_timeout": "5s",
		},
		"steps":        []any{"create_schema", "load_data"},
		"extra_params": map[string]any{"new-flag": "1"},
		"thresholds":   map[string]any{"p99_ms": 25.0, "error_rate": 0.01},
		"warmup":       "30s",
		"log_level":    "debug",
	}
	tpch := map[string]any{
		"name":     "tpch",
		"workload": map[string]any{"script": "tpch/tx", "scale_factor": 0.01, "ydb_store_mode": "row"},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	tpchRefresh := map[string]any{
		"name":     "tpch-refresh",
		"workload": map[string]any{"script": "tpch/tx", "refresh_functions": true, "refresh_set": int64(3)},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	tpcds := map[string]any{
		"name":     "tpcds",
		"workload": map[string]any{"script": "tpcds", "scale_factor": 1.0, "streams": int64(2), "query_stream": int64(1), "schema_file": "tpcds/schema.pico"},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	tpcdsBenchmark := map[string]any{
		"name":     "tpcds-benchmark",
		"workload": map[string]any{"script": "tpcds", "scale_factor": 1.0, "benchmark": true, "streams": int64(4), "query_seed": int64(42), "refresh_set": int64(3)},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	// stroppy 6.5.0 modes: the strict terminal model with its own steps, the
	// full TPC-H schedule, the TPC-DS benchmark under its mode name.
	tpccStrict := map[string]any{
		"name":     "strict",
		"workload": map[string]any{"script": "tpcc/procs", "terminal_model": "strict", "delivery_results": "delivery-1.jsonl"},
		"run":      map[string]any{"executor": "constant-vus", "vus": int64(32), "duration": "2h"},
		"steps":    []any{"create_terminal_procedures", "initialize_terminals", "workload", "delivery_results"},
	}
	tpchFull := map[string]any{
		"name":     "tpch-full",
		"workload": map[string]any{"script": "tpch/tx", "mode": "full", "scale_factor": 10.0},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	tpchPower := map[string]any{
		"name":     "tpch-power",
		"workload": map[string]any{"script": "tpch/tx", "mode": "power", "query_seed": int64(19620718), "refresh_set": int64(1)},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
		"steps":    []any{"power/prepare_streams", "power/run_1/power_queries"},
	}
	tpcdsMode := map[string]any{
		"name":     "tpcds-mode",
		"workload": map[string]any{"script": "tpcds", "mode": "benchmark", "streams": int64(4), "refresh_set": int64(2)},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
		"steps":    []any{"power", "throughput_1"},
	}
	sql := map[string]any{
		"name":     "sql",
		"workload": map[string]any{"script": "execute_sql", "sql_body": "select 1"},
		"run":      map[string]any{"executor": "constant-vus", "vus": int64(4), "duration": "10s"},
	}
	simple := map[string]any{
		"name":     "smoke",
		"workload": map[string]any{"script": "simple"},
		"run":      map[string]any{"executor": "constant-vus", "duration": "5s"},
	}

	withWorkload := func(w map[string]any) map[string]any {
		v := minimalSegment("load")
		v["workload"] = w
		return v
	}
	withRun := func(r map[string]any) map[string]any {
		v := minimalSegment("load")
		v["run"] = r
		return v
	}

	schematest.Run(t, Segment(), schematest.Cases{
		Valid: []map[string]any{minimalSegment("load"), full, tpch, tpchRefresh, tpcds, tpcdsBenchmark, tpccStrict, tpchFull, tpchPower, tpcdsMode, sql, simple},
		Invalid: []schematest.Invalid{
			{Value: func() map[string]any { v := minimalSegment("Load"); return v }(), Code: "PATTERN_MISMATCH", Path: "name"},
			{Value: withWorkload(map[string]any{"script": "ycsb"}), Code: "UNKNOWN_VARIANT", Path: "workload"},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "scale_factor": int64(0)}), Code: "GTE_VIOLATED", Path: "workload.scale_factor"},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "tx_isolation": "chaos"}), Code: "CHOICE_NOT_ALLOWED", Path: "workload.tx_isolation"},
			// tpch takes a fractional scale factor; tpcc an integer one — a
			// parameter of another script is unknown here.
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "ydb_store_mode": "row"}), Code: "UNKNOWN_FIELD", Path: "workload.ydb_store_mode"},
			{Value: withWorkload(map[string]any{"script": "execute_sql"}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "execute_sql", "sql_body": "select 1", "sql_file": "q.sql"}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "sql_dialect": "crdb24", "sql_file": "q.sql"}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "sql_dialect": "oracle"}), Code: "CHOICE_NOT_ALLOWED", Path: "workload.sql_dialect"},
			{Value: withWorkload(map[string]any{"script": "tpcb/tx", "sql_dialect": "crdb"}), Code: "UNKNOWN_FIELD", Path: "workload.sql_dialect"},
			// stroppy: "refresh-set requires refresh-functions".
			{Value: withWorkload(map[string]any{"script": "tpch/tx", "refresh_set": int64(2)}), Code: "RULE_VIOLATED", Path: ""},
			// TPC-DS benchmark mode (stroppy 6.4.1): streams even and at least 4;
			// no explicit query stream, unlogged load or custom SQL; a refresh set
			// only with it.
			{Value: withWorkload(map[string]any{"script": "tpcds", "benchmark": true, "streams": int64(3)}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "benchmark": true, "streams": int64(2)}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "benchmark": true, "query_stream": int64(0)}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "benchmark": true, "pg_unlogged": true}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "benchmark": true, "sql_file": "tpcds/pico"}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "refresh_set": int64(5)}), Code: "RULE_VIOLATED", Path: ""},
			// A step exists only in its mode's graph.
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpcc/tx"})
				v["steps"] = []any{"initialize_terminals"}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpch/tx", "mode": "full"})
				v["steps"] = []any{"workload"}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpcds", "benchmark": true})
				v["steps"] = []any{"validate_answers"}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			// Strict TPC-C: constant VUs, a transactional isolation, fixed warehouses.
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpcc/tx", "terminal_model": "strict"})
				v["run"] = map[string]any{"executor": "shared-iterations", "iterations": int64(10)}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "terminal_model": "strict", "tx_isolation": "none"}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "terminal_model": "strict", "spread_warehouses": true}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcc/tx", "terminal_model": "batch"}), Code: "CHOICE_NOT_ALLOWED", Path: "workload.terminal_model"},
			// A named TPC-H mode owns its schedule.
			{Value: withWorkload(map[string]any{"script": "tpch/tx", "mode": "full", "refresh_functions": true}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpch/tx", "mode": "power", "query_stream": int64(0)}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpch/tx", "mode": "power"}), Code: "RULE_VIOLATED", Path: ""}, // minimal run is constant-vus
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpch/tx", "mode": "throughput", "pg_unlogged": true})
				v["run"] = map[string]any{"executor": "shared-iterations", "iterations": int64(1)}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpch/tx", "mode": "full", "query_seed": int64(7)})
				v["run"] = map[string]any{"executor": "shared-iterations", "iterations": int64(1)}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: func() map[string]any {
				v := withWorkload(map[string]any{"script": "tpch/tx", "mode": "full"})
				v["run"] = map[string]any{"executor": "shared-iterations", "iterations": int64(1)}
				v["no_steps"] = []any{"full/run_2/throughput_refresh"}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			// TPC-DS benchmark mode keeps the benchmark's shape; benchmark=true is not a query mode.
			{Value: withWorkload(map[string]any{"script": "tpcds", "mode": "benchmark", "streams": int64(2)}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withWorkload(map[string]any{"script": "tpcds", "mode": "queries", "benchmark": true}), Code: "RULE_VIOLATED", Path: ""},
			{Value: withRun(map[string]any{"executor": "constant-vus"}), Code: "RULE_VIOLATED", Path: "run"},
			{Value: withRun(map[string]any{"executor": "shared-iterations", "duration": "10s"}), Code: "RULE_VIOLATED", Path: "run"},
			{Value: withRun(map[string]any{"executor": "ramping-vus", "duration": "10s"}), Code: "CHOICE_NOT_ALLOWED", Path: "run.executor"},
			{Value: withRun(map[string]any{"executor": "constant-vus", "vus": int64(0), "duration": "60s"}), Code: "GTE_VIOLATED", Path: "run.vus"},
			{Value: withRun(map[string]any{"executor": "constant-vus", "duration": "0s"}), Code: "GT_VIOLATED", Path: "run.duration"},
			{Value: func() map[string]any {
				v := minimalSegment("load")
				v["steps"] = []any{"load_data"}
				v["no_steps"] = []any{"drop_schema"}
				return v
			}(), Code: "RULE_VIOLATED", Path: ""},
			{Value: func() map[string]any {
				v := minimalSegment("load")
				v["extra_params"] = map[string]any{"Bad_Key": "1"}
				return v
			}(), Code: "RULE_VIOLATED", Path: "extra_params"},
			{Value: func() map[string]any { v := minimalSegment("load"); v["junk"] = 1; return v }(), Code: "UNKNOWN_FIELD", Path: "junk"},
		},
	})
}
