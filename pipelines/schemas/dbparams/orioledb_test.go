package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestOrioledb(t *testing.T) {
	schematest.Run(t, Orioledb(), schematest.Cases{
		Valid: []map[string]any{
			{"image_tag": "beta19-pg17"},
			// a definition stored before main_buffers_mb still loads
			{"image_tag": "beta17-pg17", "shared_buffers_mb": int64(1024)},
			{
				"image_tag":                   "latest-pg18",
				"ubuntu_base":                 true,
				"replicas":                    int64(2),
				"haproxy":                     int64(1),
				"main_buffers_mb":             int64(16384),
				"initdb_locale":               "en_US.UTF-8",
				"default_table_access_method": true,
				"init_sql":                    "CREATE EXTENSION IF NOT EXISTS orioledb;",
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"main_buffers_mb": int64(32)}, Code: "GTE_VIOLATED", Path: "main_buffers_mb"},
			{Value: map[string]any{"patroni": true}, Code: "UNKNOWN_FIELD", Path: "patroni"},
			{Value: map[string]any{"image_tag": "latest-pg15"}, Code: "CHOICE_NOT_ALLOWED", Path: "image_tag"},
			{Value: map[string]any{"haproxy": int64(3)}, Code: "LTE_VIOLATED", Path: "haproxy"},
			{Value: map[string]any{"initdb_locale": "ru RU"}, Code: "PATTERN_MISMATCH", Path: "initdb_locale"},
		},
	})
}
