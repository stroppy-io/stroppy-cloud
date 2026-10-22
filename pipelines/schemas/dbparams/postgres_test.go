package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestPostgres(t *testing.T) {
	schematest.Run(t, Postgres(), schematest.Cases{
		Valid: []map[string]any{
			{"version": "17"},
			// a definition stored before wal_archive was retired still loads
			{"version": "17", "wal_archive": true},
			{
				"version":       "18",
				"replicas":      int64(3),
				"sync_replicas": int64(1),
				"ha":            "patroni",
				"etcd_nodes":    int64(3),
				"haproxy":       int64(2),
				"pgbouncer":     true,
				"extensions":    []any{"pg_stat_statements", "pg_buffercache", "vector"},
				"locale":        "en_US.UTF-8",
				"init_sql":      "ALTER SYSTEM SET synchronous_commit = 'remote_apply';",
			},
		},
		Invalid: []schematest.Invalid{
			// range
			{Value: map[string]any{"replicas": int64(9)}, Code: "LTE_VIOLATED", Path: "replicas"},
			// unknown field
			{Value: map[string]any{"patroni": true}, Code: "UNKNOWN_FIELD", Path: "patroni"},
			// cross-field: more sync replicas than replicas
			{
				Value: map[string]any{"replicas": int64(1), "sync_replicas": int64(2)},
				Code:  "RULE_VIOLATED", Path: "sync-le-replicas",
			},
			// cross-field: Patroni with nothing to fail over to
			{
				Value: map[string]any{"ha": "patroni", "replicas": int64(0)},
				Code:  "RULE_VIOLATED", Path: "patroni-needs-replica",
			},
			// closed enums
			{Value: map[string]any{"version": "14"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
			{
				Value: map[string]any{"extensions": []any{"timescaledb"}},
				Code:  "CHOICE_NOT_ALLOWED", Path: "extensions[0]",
			},
		},
	})
}
