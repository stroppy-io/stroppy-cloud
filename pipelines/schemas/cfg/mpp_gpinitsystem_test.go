package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMppGpinitsystem1(t *testing.T) {
	schematest.Run(t, MppGpinitsystem1(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{"coordinator_keyword": "MASTER", "coordinator_hostname": "10.0.0.11", "data_directories": []any{"/data/mpp/primary", "/data/mpp/primary"}},
			{"array_name": "mpp", "seg_prefix": "seg", "port_base": int64(40000), "encoding": "LATIN1", "database_name": "bench", "coordinator_port": int64(5433)},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"coordinator_keyword": "LEADER"}, Code: "CHOICE_NOT_ALLOWED", Path: "coordinator_keyword"},
			{Value: map[string]any{"port_base": int64(80)}, Code: "GTE_VIOLATED", Path: "port_base"},
			{Value: map[string]any{"database_name": "Bad Name"}, Code: "PATTERN_MISMATCH", Path: "database_name"},
			{Value: map[string]any{"data_directories": []any{"relative/dir"}}, Code: "PATTERN_MISMATCH", Path: "data_directories[0]"},
			{Value: map[string]any{"standby": "x"}, Code: "UNKNOWN_FIELD", Path: "standby"},
		},
		Render: "conf",
		Contains: []string{
			"ARRAY_NAME=\"", "PORT_BASE=", "declare -a DATA_DIRECTORY=(", "_DIRECTORY=/data/mpp/coordinator", "TRUSTED_SHELL=ssh", "DATABASE_NAME=",
		},
	})
}
