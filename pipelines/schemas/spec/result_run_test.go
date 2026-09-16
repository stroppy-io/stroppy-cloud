package spec

import (
	"fmt"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestResultRun(t *testing.T) {
	minimal := map[string]any{}
	// A full run retains config + log per segment and one baseline log.
	artifacts := make([]any, 0, 129)
	for i := range 64 {
		artifacts = append(artifacts, fmt.Sprintf("artifact/segment-%d-config", i), fmt.Sprintf("artifact/segment-%d-log", i))
	}
	artifacts = append(artifacts, "artifact/baseline-log")

	// The wire form: the pipeline posts JSON, so nested timestamps arrive as
	// RFC3339 strings and nested durations as Go duration strings. Coercion
	// reaches nested objects and list items, not only root fields.
	full := map[string]any{
		"metrics": map[string]any{
			"tps":            map[string]any{"value": 12345.6, "unit": "tps", "min": 11000.0, "max": 13000.0, "avg": 12300.0},
			"latency_p99_ms": map[string]any{"value": 12.4, "unit": "ms"},
		},
		"segments": []any{
			map[string]any{
				"name":        "load",
				"status":      "completed",
				"started_at":  "2026-09-08T10:00:00Z",
				"finished_at": "2026-09-08T10:05:00Z",
				"metrics":     map[string]any{"rows": map[string]any{"value": 1000000.0}},
			},
			map[string]any{"name": "steady", "status": "failed", "error": "connection reset"},
		},
		"artifacts": []any{"artifact/abc", "artifact/def"},
		"summary": map[string]any{
			"tps":            12345.6,
			"latency_p50_ms": 3.1,
			"latency_p95_ms": 8.7,
			"latency_p99_ms": 12.4,
			"errors":         int64(0),
			"duration":       "5m",
		},
	}

	// Go callers (the server building a result in-process) may hand over
	// native time values for the same nested fields; both forms bake.
	native := map[string]any{
		"segments": []any{
			map[string]any{
				"name":        "load",
				"status":      "completed",
				"started_at":  time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC),
				"finished_at": time.Date(2026, 9, 8, 10, 5, 0, 0, time.UTC),
			},
		},
		"summary": map[string]any{"duration": 5 * time.Minute},
	}

	schematest.Run(t, ResultRun(), schematest.Cases{
		Valid: []map[string]any{minimal, full, native, {"artifacts": artifacts}},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"segments": []any{map[string]any{"name": "load", "status": "exploded"}},
			}, Code: "CHOICE_NOT_ALLOWED", Path: "segments[0].status"},
			{Value: map[string]any{
				"segments": []any{map[string]any{"name": "load", "status": "failed"}},
			}, Code: "RULE_VIOLATED", Path: "segments[0]"},
			{Value: map[string]any{
				"metrics": map[string]any{"tps": map[string]any{"unit": "tps"}},
			}, Code: "REQUIRED_MISSING", Path: "metrics.tps.value"},
			{Value: map[string]any{
				"summary": map[string]any{"errors": int64(-1)},
			}, Code: "GTE_VIOLATED", Path: "summary.errors"},
			// A nested duration below zero: the bound is checked after the
			// nested value is coerced from its string form.
			{Value: map[string]any{
				"summary": map[string]any{"duration": "-1s"},
			}, Code: "GTE_VIOLATED", Path: "summary.duration"},
			{Value: map[string]any{"artifacts": []any{"a", "a"}}, Code: "NOT_UNIQUE", Path: "artifacts[1]"},
		},
	})
}
