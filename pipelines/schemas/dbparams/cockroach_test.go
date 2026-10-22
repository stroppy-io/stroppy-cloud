package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestCockroach(t *testing.T) {
	schematest.Run(t, Cockroach(), schematest.Cases{
		Valid: []map[string]any{
			{"version": "25.4"},
			{
				"version": "26.3",
				"nodes":   int64(3),
				"locality": []any{
					"region=ru-central1,zone=ru-central1-a",
					"region=ru-central1,zone=ru-central1-b",
					"region=ru-central1,zone=ru-central1-d",
				},
				"insecure": true,
				"haproxy":  int64(1),
				"init_sql": "SET CLUSTER SETTING kv.range_merge.queue_enabled = false;",
			},
		},
		Invalid: []schematest.Invalid{
			// range
			{Value: map[string]any{"nodes": int64(21)}, Code: "LTE_VIOLATED", Path: "nodes"},
			// unknown field
			{Value: map[string]any{"options": map[string]any{}}, Code: "UNKNOWN_FIELD", Path: "options"},
			// cross-field: an even cluster buys nothing over the odd one below it
			{Value: map[string]any{"nodes": int64(4)}, Code: "RULE_VIOLATED", Path: "odd-nodes"},
			// cross-field: locality must cover every node
			{
				Value: map[string]any{"nodes": int64(3), "locality": []any{"region=eu"}},
				Code:  "RULE_VIOLATED", Path: "locality-per-node",
			},
			// no certificates are provisioned, so secure mode is refused
			{Value: map[string]any{"insecure": false}, Code: "RULE_VIOLATED", Path: "insecure-only"},
			// EOL versions are gone
			{Value: map[string]any{"version": "23.2"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
			{
				Value: map[string]any{"nodes": int64(1), "locality": []any{"just-a-zone"}},
				Code:  "PATTERN_MISMATCH", Path: "locality[0]",
			},
		},
	})
}
