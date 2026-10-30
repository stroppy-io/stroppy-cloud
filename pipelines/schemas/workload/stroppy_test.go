package workload

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestStroppy(t *testing.T) {
	minimal := map[string]any{
		"stroppy_version": "6.3.0",
		"protocol":        "pg",
		"segments":        []any{minimalSegment("load")},
	}
	full := map[string]any{
		"stroppy_version": "nightly-24898a8",
		"protocol":        "ydb_grpcs",
		"segments":        []any{minimalSegment("load"), minimalSegment("steady")},
		"driver": map[string]any{
			"default_insert_method": "native",
			"bulk_size":             int64(5000),
			"pool":                  map[string]any{"max_conns": int64(64), "min_conns": int64(8), "max_conn_lifetime": "1h"},
			"insert_progress":       map[string]any{"mode": "both", "interval": "30s", "stall_after": "2m"},
		},
		"connection": map[string]any{
			"kind":       "ydb",
			"auth_token": "t1.secret",
		},
		"baseline": map[string]any{"enabled": true, "tiers": []any{"noop"}, "quick": true, "vus": int64(8)},
	}
	pg := map[string]any{
		"stroppy_version": "6.1.0-rc.1",
		"protocol":        "pg",
		"segments":        []any{minimalSegment("load")},
		"connection":      map[string]any{"kind": "pg", "sslmode": "require", "query_exec_mode": "simple_protocol"},
	}

	with := func(over map[string]any) map[string]any {
		v := map[string]any{"stroppy_version": "6.3.0", "protocol": "pg", "segments": []any{minimalSegment("load")}}
		for k, val := range over {
			v[k] = val
		}
		return v
	}

	schematest.Run(t, Stroppy(), schematest.Cases{
		Valid: []map[string]any{minimal, full, pg},
		Invalid: []schematest.Invalid{
			{Value: with(map[string]any{"stroppy_version": "v6"}), Code: "PATTERN_MISMATCH", Path: "stroppy_version"},
			// The Go-native engine starts at 6.0.0; older builds are not run.
			{Value: with(map[string]any{"stroppy_version": "5.7.3"}), Code: "RULE_VIOLATED", Path: "stroppy_version"},
			{Value: with(map[string]any{"protocol": "oracle"}), Code: "CHOICE_NOT_ALLOWED", Path: "protocol"},
			{Value: with(map[string]any{"segments": []any{}}), Code: "MIN_ITEMS_VIOLATED", Path: "segments"},
			{Value: with(map[string]any{"connection": map[string]any{"kind": "mysql"}}), Code: "RULE_VIOLATED", Path: ""},
			{Value: with(map[string]any{"segments": []any{minimalSegment("load"), minimalSegment("load")}}), Code: "RULE_VIOLATED", Path: ""},
			{Value: with(map[string]any{"driver": map[string]any{"default_insert_method": "copy"}}), Code: "CHOICE_NOT_ALLOWED", Path: "driver.default_insert_method"},
			{Value: with(map[string]any{"baseline": map[string]any{"enabled": true, "tiers": []any{"gpu"}}}), Code: "CHOICE_NOT_ALLOWED", Path: "baseline.tiers[0]"},
			{Value: with(map[string]any{"protocol": "mysql", "driver": map[string]any{"default_insert_method": "columnar"}}), Code: "RULE_VIOLATED", Path: ""},
		},
	})
}
