package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func mariadbFull() map[string]any {
	return map[string]any{
		"innodb_buffer_pool_size":         int64(16384),
		"innodb_log_file_size":            int64(4096),
		"innodb_io_capacity":              int64(2000),
		"innodb_io_capacity_max":          int64(4000),
		"innodb_autoinc_lock_mode":        int64(2),
		"binlog_format":                   "ROW",
		"log_slave_updates":               "ON",
		"gtid_strict_mode":                "ON",
		"gtid_domain_id":                  int64(7),
		"rpl_semi_sync_master_enabled":    "ON",
		"rpl_semi_sync_master_wait_point": "AFTER_SYNC",
		"slave_parallel_threads":          int64(8),
		"wsrep_on":                        "ON",
		"wsrep_provider":                  "/usr/lib/galera/libgalera_smm.so",
		"wsrep_cluster_name":              "stroppy",
		"wsrep_cluster_address":           "gcomm://10.0.0.1,10.0.0.2,10.0.0.3",
		"wsrep_node_address":              "10.0.0.1",
		"wsrep_node_name":                 "mdb-1",
		"wsrep_sst_method":                "mariabackup",
		"wsrep_slave_threads":             int64(4),
		"max_connections":                 int64(4000),
		"tmp_table_size":                  int64(64),
		"max_heap_table_size":             int64(64),
		"server_id":                       int64(11),
		"report_host":                     "mdb-1.stroppy.internal",
		"custom":                          map[string]any{"innodb_purge_threads": "8"},
	}
}

func TestMariadbCnf1011(t *testing.T) {
	full := mariadbFull()
	full["tx_isolation"] = "READ-COMMITTED"
	full["innodb_flush_method"] = "O_DIRECT"
	full["innodb_change_buffering"] = "none"
	full["innodb_doublewrite"] = "ON"

	schematest.Run(t, MariadbCnf1011(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"transaction_isolation": "REPEATABLE-READ"}, Code: "UNKNOWN_FIELD", Path: "transaction_isolation"},
			{Value: map[string]any{"innodb_thread_concurrency": int64(0)}, Code: "UNKNOWN_FIELD", Path: "innodb_thread_concurrency"},
			{Value: map[string]any{"innodb_doublewrite": "fast"}, Code: "CHOICE_NOT_ALLOWED", Path: "innodb_doublewrite"},
			{Value: map[string]any{"wsrep_on": "ON", "wsrep_provider": "/x.so", "innodb_autoinc_lock_mode": int64(1), "binlog_format": "ROW"}, Code: "RULE_VIOLATED", Path: "galera-autoinc-lock-mode"},
		},
		Render: "conf",
		Contains: []string{
			"[mariadb]",
			"innodb_log_file_size = ",
			"transaction-isolation = ",
			"log_warnings = ",
			"wsrep_on = ",
		},
	})
}

func TestMariadbCnf1104(t *testing.T) {
	full := mariadbFull()
	full["transaction_isolation"] = "READ-COMMITTED"
	full["innodb_doublewrite"] = "fast"
	full["innodb_log_file_buffering"] = "OFF"
	full["innodb_data_file_buffering"] = "OFF"

	schematest.Run(t, MariadbCnf1104(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"tx_isolation": "REPEATABLE-READ"}, Code: "UNKNOWN_FIELD", Path: "tx_isolation"},
			{Value: map[string]any{"innodb_change_buffering": "none"}, Code: "UNKNOWN_FIELD", Path: "innodb_change_buffering"},
			{Value: map[string]any{"wsrep_slave_threads": int64(0)}, Code: "GTE_VIOLATED", Path: "wsrep_slave_threads"},
			{Value: map[string]any{"wsrep_on": "ON", "wsrep_provider": "/x.so", "innodb_autoinc_lock_mode": int64(2), "binlog_format": "MIXED"}, Code: "RULE_VIOLATED", Path: "galera-binlog-format"},
		},
		Render: "conf",
		Contains: []string{
			"[mariadb]",
			"transaction_isolation = ",
			"innodb_data_file_buffering = ",
		},
	})
}

func TestMariadbCnf1108(t *testing.T) {
	full := mariadbFull()
	full["transaction_isolation"] = "READ-COMMITTED"
	full["innodb_doublewrite"] = "fast"
	full["innodb_log_file_buffering"] = "OFF"
	full["innodb_data_file_buffering"] = "OFF"
	full["innodb_snapshot_isolation"] = "OFF"
	full["max_tmp_session_space_usage"] = int64(4096)
	full["max_tmp_total_space_usage"] = int64(65536)
	full["log_slow_always_query_time"] = int64(10)
	full["slave_abort_blocking_timeout"] = int64(60)

	schematest.Run(t, MariadbCnf1108(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"tx_isolation": "REPEATABLE-READ"}, Code: "UNKNOWN_FIELD", Path: "tx_isolation"},
			{Value: map[string]any{"innodb_change_buffering": "none"}, Code: "UNKNOWN_FIELD", Path: "innodb_change_buffering"},
			{Value: map[string]any{"innodb_snapshot_isolation": "fast"}, Code: "CHOICE_NOT_ALLOWED", Path: "innodb_snapshot_isolation"},
			{Value: map[string]any{"max_tmp_total_space_usage": int64(-1)}, Code: "GTE_VIOLATED", Path: "max_tmp_total_space_usage"},
		},
		Render: "conf",
		Contains: []string{
			"[mariadb]",
			"transaction_isolation = ",
			"innodb_data_file_buffering = ",
			// 11.8 additions
			"innodb_snapshot_isolation = ",
			"max_tmp_total_space_usage = ",
			"log_slow_always_query_time = ",
			"slave_abort_blocking_timeout = ",
		},
	})

	// 11.4 must not have grown the 11.8 knobs.
	out := renderDefaults(t, MariadbCnf1104())
	dontWantLines(t, out, "innodb_snapshot_isolation", "max_tmp_total_space_usage",
		"log_slow_always_query_time", "slave_abort_blocking_timeout")
}
