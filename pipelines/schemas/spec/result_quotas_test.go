package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestResultQuotas(t *testing.T) {
	schematest.Run(t, ResultQuotas(), schematest.Cases{
		Valid: []map[string]any{
			{"observed_at": "2026-09-12T10:00:00Z", "unavailable_reason": "permission_denied", "scope": "cloud:b1g", "quotas": []any{}},
			{
				"observed_at": "2026-09-08T10:00:00Z",
				"quotas": []any{
					map[string]any{"name": "compute.instances.count", "limit": 12.0, "used": 4.0, "unit": "count"},
				},
			},
			{
				"observed_at": "2026-09-08T10:00:00Z",
				"quotas": []any{
					map[string]any{"name": "compute.instanceCores.count", "limit": 96.0, "used": 24.0, "unit": "count"},
					map[string]any{
						"name": "compute.ssdDisks.size", "limit": 2048.0, "used": 300.0,
						"unit": "GB", "zone": "ru-central1-d",
					},
				},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"quotas": []any{}}, Code: "REQUIRED_MISSING", Path: "observed_at"},
			{
				Value: map[string]any{"observed_at": "2026-09-08T10:00:00Z"},
				Code:  "REQUIRED_MISSING", Path: "quotas",
			},
			{Value: map[string]any{
				"observed_at": "2026-09-08T10:00:00Z",
				"quotas":      []any{map[string]any{"name": "compute.instances.count", "limit": -1.0, "used": 0.0}},
			}, Code: "GTE_VIOLATED", Path: "quotas[0].limit"},
			{Value: map[string]any{
				"observed_at": "2026-09-08T10:00:00Z",
				"quotas":      []any{map[string]any{"limit": 1.0, "used": 0.0}},
			}, Code: "REQUIRED_MISSING", Path: "quotas[0].name"},
		},
	})
}
