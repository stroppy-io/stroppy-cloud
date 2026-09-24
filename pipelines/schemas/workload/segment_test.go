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
			"sql_file":        "tpcc/ydb_no_indexes",
		},
		"run": map[string]any{
			"executor":      "shared-iterations",
			"vus":           int64(64),
			"iterations":    int64(100000),
			"query_timeout": "5s",
		},
		"steps":        []any{"create_schema", "load_data"},
		"extra_params": map[string]any{"new-flag": "1"},
		"thresholds": map[string]any{"p99_ms": 25.0, "error_rate": 0.01},
		"warmup":     "30s",
		"log_level":  "debug",
	}
	tpch := map[string]any{
		"name":     "tpch",
		"workload": map[string]any{"script": "tpch/tx", "scale_factor": 0.01, "ydb_store_mode": "row"},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
	}
	tpcds := map[string]any{
		"name":     "tpcds",
		"workload": map[string]any{"script": "tpcds", "scale_factor": 1.0, "streams": int64(2), "query_stream": int64(1), "schema_file": "tpcds/schema.pico"},
		"run":      map[string]any{"executor": "shared-iterations", "iterations": int64(1)},
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
		Valid: []map[string]any{minimalSegment("load"), full, tpch, tpcds, sql, simple},
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
