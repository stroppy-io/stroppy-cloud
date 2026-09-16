package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMyCnfGroupReplicationRender(t *testing.T) {
	for _, s := range []*schemapb.Schema{MyCnf80(), MyCnf84()} {
		schematest.Run(t, s, schematest.Cases{
			Valid: []map[string]any{myFull()}, Render: "conf",
			Contains: []string{
				"plugin_load_add = group_replication.so",
				"loose-group_replication_group_name = aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"loose-group_replication_local_address = 10.0.0.1:33061",
				"loose-group_replication_group_seeds = 10.0.0.1:33061,10.0.0.2:33061",
			},
		})
	}
}

func myFull() map[string]any {
	return map[string]any{
		"innodb_buffer_pool_size":         int64(16384),
		"innodb_buffer_pool_instances":    int64(8),
		"innodb_redo_log_capacity":        int64(8192),
		"innodb_flush_method":             "O_DIRECT",
		"innodb_io_capacity":              int64(20000),
		"innodb_io_capacity_max":          int64(40000),
		"binlog_format":                   "ROW",
		"gtid_mode":                       "ON",
		"enforce_gtid_consistency":        "ON",
		"log_replica_updates":             "ON",
		"rpl_semi_sync_source_enabled":    "ON",
		"rpl_semi_sync_source_timeout":    int64(1000),
		"read_only":                       "ON",
		"super_read_only":                 "ON",
		"group_replication":               true,
		"group_replication_group_name":    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"group_replication_local_address": "10.0.0.1:33061",
		"group_replication_group_seeds":   "10.0.0.1:33061,10.0.0.2:33061",
		"max_connections":                 int64(4000),
		"tmp_table_size":                  int64(64),
		"max_heap_table_size":             int64(64),
		"server_id":                       int64(101),
		"report_host":                     "db-1.stroppy.internal",
		"custom":                          map[string]any{"innodb_purge_threads": "8"},
	}
}

func TestMyCnf80(t *testing.T) {
	full := myFull()
	full["replica_parallel_type"] = "LOGICAL_CLOCK"
	full["transaction_write_set_extraction"] = "XXHASH64"
	full["default_authentication_plugin"] = "caching_sha2_password"

	schematest.Run(t, MyCnf80(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"innodb_buffer_pool_size": int64(1)}, Code: "GTE_VIOLATED", Path: "innodb_buffer_pool_size"},
			{Value: map[string]any{"authentication_policy": "*,,"}, Code: "UNKNOWN_FIELD", Path: "authentication_policy"},
			{Value: map[string]any{"transaction_isolation": "SNAPSHOT"}, Code: "CHOICE_NOT_ALLOWED", Path: "transaction_isolation"},
			{Value: map[string]any{"lower_case_table_names": int64(3)}, Code: "NOT_IN_ALLOWED_SET", Path: "lower_case_table_names"},
			{Value: map[string]any{"custom": map[string]any{"BAD KEY": "1"}}, Code: "RULE_VIOLATED"},
		},
		Render: "conf",
		Contains: []string{
			"[mysqld]",
			"innodb_buffer_pool_size = ",
			"binlog_format = ",
			"default_authentication_plugin = ",
			"replica_parallel_type = ",
		},
	})
}

func TestMyCnf84(t *testing.T) {
	full := myFull()
	full["authentication_policy"] = "*,,"
	full["mysql_native_password"] = "OFF"

	schematest.Run(t, MyCnf84(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"default_authentication_plugin": "caching_sha2_password"}, Code: "UNKNOWN_FIELD", Path: "default_authentication_plugin"},
			{Value: map[string]any{"transaction_write_set_extraction": "XXHASH64"}, Code: "UNKNOWN_FIELD", Path: "transaction_write_set_extraction"},
			{Value: map[string]any{"replica_parallel_type": "DATABASE"}, Code: "UNKNOWN_FIELD", Path: "replica_parallel_type"},
			{Value: map[string]any{"port": int64(0)}, Code: "GTE_VIOLATED", Path: "port"},
		},
		Render: "conf",
		Contains: []string{
			"[mysqld]",
			"authentication_policy = ",
			"innodb_redo_log_capacity = ",
		},
	})
}

func TestMyCnfSemisyncTimeoutRender(t *testing.T) {
	for _, schema := range []*schemapb.Schema{MyCnf80(), MyCnf84()} {
		schematest.Run(t, schema, schematest.Cases{
			Valid:  []map[string]any{{"rpl_semi_sync_source_enabled": "ON", "rpl_semi_sync_source_timeout": int64(1733)}},
			Render: "conf", Contains: []string{"loose-rpl_semi_sync_source_timeout = 1733"},
		})
	}
}
