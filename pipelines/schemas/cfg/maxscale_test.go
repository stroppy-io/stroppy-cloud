package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMaxscale24(t *testing.T) {
	schematest.Run(t, Maxscale24(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"threads":                    "8",
				"admin_host":                 "0.0.0.0",
				"monitor_module":             "mariadbmon",
				"monitor_password":           "mon-pass",
				"monitor_interval":           int64(1000),
				"backend_timeout":            int64(5000),
				"auto_failover":              "true",
				"auto_rejoin":                "true",
				"enforce_read_only_slaves":   "true",
				"replication_password":       "repl-pass",
				"service_password":           "svc-pass",
				"causal_reads":               "fast_universal",
				"transaction_replay":         "true",
				"transaction_replay_timeout": int64(60000),
				"master_failure_mode":        "fail_on_write",
				"slave_selection_criteria":   "adaptive_routing",
				"max_replication_lag":        int64(30),
				"servers": []any{
					map[string]any{"name": "db-1", "address": "10.0.0.1"},
					map[string]any{"name": "db-2", "address": "10.0.0.2", "port": int64(3307), "priority": int64(2)},
				},
				"custom": map[string]any{"connection_keepalive": "300s"},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"slave_selection_criteria": "LEAST_CURRENT_OPERATIONS"}, Code: "CHOICE_NOT_ALLOWED", Path: "slave_selection_criteria"},
			{Value: map[string]any{"transaction_replay": "true", "master_failure_mode": "fail_instantly"}, Code: "RULE_VIOLATED", Path: "replay-failure-mode"},
			{Value: map[string]any{"monitor_module": "galeramon", "auto_failover": "true"}, Code: "RULE_VIOLATED", Path: "galera-no-failover"},
			{Value: map[string]any{"threads": "many"}, Code: "PATTERN_MISMATCH", Path: "threads"},
			// One backend_timeout field sets all three monitor timeouts.
			{Value: map[string]any{"backend_connect_timeout": int64(3000)}, Code: "UNKNOWN_FIELD", Path: "backend_connect_timeout"},
			{Value: map[string]any{"backend_timeout": int64(100)}, Code: "GTE_VIOLATED", Path: "backend_timeout"},
			{Value: map[string]any{"servers": []any{map[string]any{"address": "10.0.0.1", "protocol": "MariaDBBackend"}}}, Code: "UNKNOWN_FIELD", Path: "servers[0].protocol"},
		},
		Render: "conf",
		Contains: []string{
			"[maxscale]",
			"router=readwritesplit",
			"monitor_interval=",
			"backend_connect_timeout=",
			"transaction_replay_timeout=",
			"type=listener",
		},
	})

	out := renderDefaults(t, Maxscale24())
	wantLines(t, out,
		"backend_connect_timeout=3000ms",
		"backend_read_timeout=3000ms",
		"backend_write_timeout=3000ms",
		"transaction_replay_timeout=30000ms",
		// 24.02 moved the default here.
		"master_failure_mode=fail_on_write",
	)
	dontWantLines(t, out, "backend_timeout=")
}
