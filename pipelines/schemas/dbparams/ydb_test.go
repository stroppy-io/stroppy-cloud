package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestYdb(t *testing.T) {
	schematest.Run(t, Ydb(), schematest.Cases{
		Valid: []map[string]any{
			{"version": "26.2"},
			// params saved before failure_domain and grpcs left the form
			{"version": "26.2", "failure_domain": "disk", "grpcs": true, "storage_groups": int64(1)},
			{"version": "26.2", "tuning": "stock", "disk_type": "HDD", "auto_size_pdisks": false},
			{
				"version":          "26.3",
				"fault_tolerance":  "mirror-3-dc",
				"storage_nodes":    int64(9),
				"database_nodes":   int64(3),
				"pdisks_per_node":  int64(3),
				"disk_type":        "NVME",
				"storage_groups":   int64(4),
				"auto_size_pdisks": false,
				"database_path":    "/Root/stroppy-mirror",
				"haproxy":          int64(2),
			},
		},
		Invalid: []schematest.Invalid{
			// range
			{Value: map[string]any{"pdisks_per_node": int64(9)}, Code: "LTE_VIOLATED", Path: "pdisks_per_node"},
			// unknown field (the old proto spelling)
			{
				Value: map[string]any{"pdisks_per_storage_node": int64(3)},
				Code:  "UNKNOWN_FIELD", Path: "pdisks_per_storage_node",
			},
			// cross-field: block-4-2 on a single pdisk
			{
				Value: map[string]any{"fault_tolerance": "block-4-2", "storage_nodes": int64(1), "pdisks_per_node": int64(1)},
				Code:  "RULE_VIOLATED", Path: "block42-min-domains",
			},
			// cross-field: mirror-3-dc with too few domains (pdisks of one
			// node are not separate domains)
			{
				Value: map[string]any{"fault_tolerance": "mirror-3-dc", "storage_nodes": int64(3), "pdisks_per_node": int64(3)},
				Code:  "RULE_VIOLATED", Path: "mirror3dc-min-domains",
			},
			// cross-field: mirror-3-dc hosts must split over 3 realms
			{
				Value: map[string]any{"fault_tolerance": "mirror-3-dc", "storage_nodes": int64(10), "pdisks_per_node": int64(3)},
				Code:  "RULE_VIOLATED", Path: "mirror3dc-three-realms",
			},
			{Value: map[string]any{"storage_groups": int64(0)}, Code: "GTE_VIOLATED", Path: "storage_groups"},
			// the pre-2026 ROT spelling is gone
			{Value: map[string]any{"disk_type": "ROT"}, Code: "CHOICE_NOT_ALLOWED", Path: "disk_type"},
			// /Root itself is the cluster root, not a database
			{Value: map[string]any{"database_path": "/local"}, Code: "PATTERN_MISMATCH", Path: "database_path"},
		},
	})
}
