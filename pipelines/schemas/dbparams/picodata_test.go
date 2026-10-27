package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func picoTier(name string, instances, rf int64) map[string]any {
	return map[string]any{"name": name, "instances": instances, "replication_factor": rf}
}

func TestPicodata(t *testing.T) {
	schematest.Run(t, Picodata(), schematest.Cases{
		Valid: []map[string]any{
			{"tiers": []any{picoTier("default", 1, 1)}},
			{
				"version": "26.2",
				"tiers": []any{
					map[string]any{
						"name": "default", "instances": int64(6), "replication_factor": int64(2),
						"can_vote": true, "bucket_count": int64(6000), "replication_mode": "sync",
					},
					map[string]any{
						"name": "router", "instances": int64(2), "replication_factor": int64(1),
						"can_vote": false,
					},
				},
				"default_bucket_count": int64(6000),
				"memtx_memory_mb":      int64(8192),
				"haproxy":              int64(1),
				"pgproto":              true,
			},
			{
				"version": "26.2",
				"tiers": []any{map[string]any{
					"name": "default", "instances": int64(12), "replication_factor": int64(3), "wal_mode": "fsync",
				}},
				"instances_per_machine":         int64(4),
				"sql_runtime_concurrency_max":   int64(192),
				"sql_storage_cache_count_max":   int64(200),
				"sql_storage_cache_size_max_mb": int64(32),
				"sql_motion_row_max":            int64(50000),
			},
			{"version": "26.1", "tiers": []any{map[string]any{"name": "default", "instances": int64(1), "wal_mode": "write"}}},
		},
		Invalid: []schematest.Invalid{
			// range: memtx below the engine minimum
			{
				Value: map[string]any{"tiers": []any{picoTier("default", 1, 1)}, "memtx_memory_mb": int64(8)},
				Code:  "GTE_VIOLATED", Path: "memtx_memory_mb",
			},
			// unknown field
			{
				Value: map[string]any{"tiers": []any{picoTier("default", 1, 1)}, "instances": int64(3)},
				Code:  "UNKNOWN_FIELD", Path: "instances",
			},
			// unknown field inside a tier
			{
				Value: map[string]any{"tiers": []any{map[string]any{"name": "default", "count": int64(3)}}},
				Code:  "UNKNOWN_FIELD", Path: "tiers[0].count",
			},
			// cross-field: no tier named default
			{
				Value: map[string]any{"tiers": []any{picoTier("hot", 1, 1)}},
				Code:  "RULE_VIOLATED", Path: "default-tier-required",
			},
			// cross-field: instances do not divide into whole replicasets
			{
				Value: map[string]any{"tiers": []any{picoTier("default", 5, 2)}},
				Code:  "RULE_VIOLATED", Path: "tier-replicasets-whole",
			},
			// cross-field: duplicate tier names
			{
				Value: map[string]any{"tiers": []any{picoTier("default", 1, 1), picoTier("default", 1, 1)}},
				Code:  "RULE_VIOLATED", Path: "tier-names-unique",
			},
			// cross-field: fsync WAL before 26.2
			{Value: map[string]any{"version": "26.1", "tiers": []any{
				map[string]any{"name": "default", "instances": int64(1), "wal_mode": "fsync"},
			}}, Code: "RULE_VIOLATED", Path: "fsync-requires-26-2"},
			// range: more instances per machine than the port layout allows
			{
				Value: map[string]any{"tiers": []any{picoTier("default", 1, 1)}, "instances_per_machine": int64(129)},
				Code:  "LTE_VIOLATED", Path: "instances_per_machine",
			},
			// cross-field: nobody can vote
			{Value: map[string]any{"tiers": []any{
				map[string]any{"name": "default", "instances": int64(1), "replication_factor": int64(1), "can_vote": false},
			}}, Code: "RULE_VIOLATED", Path: "some-tier-votes"},
		},
	})
}
