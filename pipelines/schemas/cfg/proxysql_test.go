package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestProxysqlClusterRender(t *testing.T) {
	for _, topology := range []string{"group_replication", "galera"} {
		t.Run(topology, func(t *testing.T) {
			schematest.Run(t, Proxysql2(), schematest.Cases{
				Valid: []map[string]any{{"topology": topology, "restapi_enabled": "true", "mysql_servers": []any{map[string]any{"address": "10.0.0.1", "hostgroup": int64(10), "use_ssl": int64(1)}}}},
				Invalid: []schematest.Invalid{
					{Value: map[string]any{"topology": topology, "backup_writer_hostgroup": int64(10)}, Code: "RULE_VIOLATED", Path: "gr-hostgroups-distinct"},
					{Value: map[string]any{"topology": topology, "offline_hostgroup": int64(20)}, Code: "RULE_VIOLATED", Path: "gr-hostgroups-distinct"},
				},
				Render: "conf", Contains: []string{"mysql_" + topology + "_hostgroups =", "restapi_enabled=true", "restapi_port=6070", "use_ssl=1"},
			})
		})
	}
}

func TestProxysql2(t *testing.T) {
	schematest.Run(t, Proxysql2(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"datadir":                 "/data/proxysql",
				"admin_credentials":       "admin:s3cret",
				"threads":                 int64(16),
				"max_connections":         int64(20000),
				"server_version":          "8.4.0",
				"monitor_password":        "monitor-pass",
				"username":                "stroppy",
				"password":                "stroppy-pass",
				"topology":                "group_replication",
				"writer_hostgroup":        int64(10),
				"reader_hostgroup":        int64(20),
				"max_transactions_behind": int64(100),
				"writer_is_also_reader":   int64(2),
				"mysql_servers": []any{
					map[string]any{"address": "10.0.0.1", "hostgroup": int64(10), "max_connections": int64(500)},
					map[string]any{"address": "10.0.0.2", "hostgroup": int64(20), "port": int64(3306), "weight": int64(2)},
				},
				"custom": map[string]any{"have_compress": "true"},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"writer_hostgroup": int64(5), "reader_hostgroup": int64(5)}, Code: "RULE_VIOLATED", Path: "hostgroups-distinct"},
			{Value: map[string]any{"threads": int64(0)}, Code: "GTE_VIOLATED", Path: "threads"},
			{Value: map[string]any{"check_type": "read_only|innodb_read_only"}, Code: "CHOICE_NOT_ALLOWED", Path: "check_type"},
			{Value: map[string]any{"mysql_servers": []any{map[string]any{"port": int64(3306)}}}, Code: "REQUIRED_MISSING", Path: "mysql_servers[0].address"},
			{Value: map[string]any{"monitor_read_only_timeout": int64(2000), "monitor_read_only_interval": int64(1500)}, Code: "RULE_VIOLATED", Path: "read-only-timeout"},
		},
		Render: "conf",
		Contains: []string{
			"admin_variables=",
			"mysql_variables=",
			"mysql_users =",
			"mysql_query_rules =",
			"monitor_read_only_interval=",
			`default_charset="utf8mb4"`,
			`default_collation_connection="utf8mb4_general_ci"`,
		},
	})
}
