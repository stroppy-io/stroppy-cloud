package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func script(id string) map[string]any {
	return map[string]any{
		"id":        id,
		"title":     "TPC-C, raw transactions",
		"protocols": []any{"pg", "mysql"},
		"steps": []any{
			map[string]any{"id": "drop_schema", "phase": "bootstrap"},
			map[string]any{"id": "create_schema", "phase": "bootstrap"},
			map[string]any{"id": "load_data", "phase": "bootstrap"},
			map[string]any{"id": "workload_tx_new_order", "phase": "workload"},
			map[string]any{"id": "full/run_1/power_queries", "phase": "workload", "modes": []any{"full"}},
		},
		"params": []any{
			map[string]any{"name": "scale-factor", "config": "scaleFactor", "type": "int", "default": int64(1), "description": "Number of warehouses.", "env": "SCALE_FACTOR"},
			map[string]any{"name": "load-items", "config": "loadItems", "type": "bool", "default": nil, "default_description": "true when warehouse-start is 1; false otherwise"},
			map[string]any{"name": "duration", "config": "duration", "scope": "run", "type": "duration", "default": "0s"},
		},
	}
}

func version(v string, def bool) map[string]any {
	return map[string]any{
		"version":   v,
		"image":     "ghcr.io/stroppy-io/stroppy:v" + v + ".62",
		"default":   def,
		"protocols": []any{"pg", "mysql", "ydb_grpc", "noop"},
		"scripts":   []any{script("tpcc/tx")},
	}
}

func TestStroppyCatalog(t *testing.T) {
	minimal := map[string]any{"versions": []any{version("6.0.0", true)}}

	full := map[string]any{
		"source": "release",
		"versions": []any{
			version("6.0.0", true),
			map[string]any{
				"version":    "nightly-24898a8",
				"image":      "cr.stroppy.io/stroppy:nightly-24898a8",
				"deprecated": true,
				"baseline":   true,
				"protocols":  []any{"pg"},
				"scripts": []any{
					script("tpcb/procs"),
					map[string]any{
						"id":          "tpch/tx",
						"title":       "TPC-H",
						"description": "Relational load of eight tables plus the 22 query suite.",
						"steps":       []any{map[string]any{"id": "load_data", "phase": "bootstrap"}},
					},
				},
			},
		},
	}

	schematest.Run(t, StroppyCatalog(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"versions": []any{version("6.0.0", false)}},
				Code:  "RULE_VIOLATED", Path: "",
			},
			{
				Value: map[string]any{"versions": []any{version("6.0.0", true), version("6.1.0", true)}},
				Code:  "RULE_VIOLATED", Path: "",
			},
			{
				Value: map[string]any{"versions": []any{version("v6", true)}},
				Code:  "PATTERN_MISMATCH", Path: "versions[0].version",
			},
			{
				Value: map[string]any{"versions": []any{version("5.7.3", true)}},
				Code:  "RULE_VIOLATED", Path: "versions[0].version",
			},
			{Value: map[string]any{"versions": []any{
				map[string]any{
					"version": "6.0.0", "image": "x", "default": true,
					"protocols": []any{"oracle"}, "scripts": []any{script("tpcc/tx")},
				},
			}}, Code: "CHOICE_NOT_ALLOWED", Path: "versions[0].protocols[0]"},
			{Value: map[string]any{"versions": []any{}}, Code: "MIN_ITEMS_VIOLATED", Path: "versions"},
		},
	})
}
