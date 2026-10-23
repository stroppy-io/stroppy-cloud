package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestExporterMysqld(t *testing.T) {
	schematest.Run(t, ExporterMysqld(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"mysqld_address":               "10.0.0.1:3306",
				"mysqld_username":              "exporter",
				"mysqld_password":              "exporter-pass",
				"listen_port":                  int64(9105),
				"log_level":                    "debug",
				"query_timeout":                int64(10),
				"info_schema_innodb_metrics":   "ON",
				"perf_schema_eventsstatements": "ON",
				"engine_innodb_status":         "ON",
				"info_schema_processlist":      "OFF",
				"custom":                       map[string]any{"exporter.lock_wait_timeout": "5"},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"listen_port": int64(0)}, Code: "GTE_VIOLATED", Path: "listen_port"},
			{Value: map[string]any{"log_level": "trace"}, Code: "CHOICE_NOT_ALLOWED", Path: "log_level"},
			{Value: map[string]any{"collect.global_status": "ON"}, Code: "UNKNOWN_FIELD", Path: `["collect.global_status"]`},
			{Value: map[string]any{"global_status": "yes"}, Code: "CHOICE_NOT_ALLOWED", Path: "global_status"},
		},
		Render: "conf",
		Contains: []string{
			"--web.listen-address=:",
			"--collect.global_status",
			"--no-collect.info_schema.processlist",
			"--log.level=",
		},
	})
}
