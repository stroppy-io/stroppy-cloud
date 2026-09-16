package provider

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestYandexSettings(t *testing.T) {
	minimal := map[string]any{
		"cloud_id":  "b1glku4lgd6gabcdefgh",
		"folder_id": "b1gia87mbaomkfvsleds",
		"zone":      "ru-central1-d",
		"network":   map[string]any{"kind": "create"},
	}
	full := map[string]any{
		"cloud_id":    "b1glku4lgd6gabcdefgh",
		"folder_id":   "b1gia87mbaomkfvsleds",
		"zone":        "ru-central1-b",
		"platform_id": "highfreq-v3",
		"network": map[string]any{
			"kind":              "existing",
			"network_id":        "enp2v5nl4h1mabcdefgh",
			"subnet_id":         "e9bnc0k8t9klabcdefgh",
			"security_group_id": "enpq4v6ba7uhabcdefgh",
		},
		"public_ips":   false,
		"image_family": "ubuntu-2204-lts",
		"preemptible":  true,
	}

	schematest.Run(t, YandexSettings(), schematest.Cases{
		Valid: []map[string]any{minimal},
		Invalid: []schematest.Invalid{
			{Value: full, Code: "RULE_VIOLATED", Path: "network-create-only"},
			{Value: map[string]any{
				"cloud_id":  "b1glku4lgd6gabcdefgh",
				"folder_id": "not-a-folder",
				"zone":      "ru-central1-d",
				"network":   map[string]any{"kind": "create"},
			}, Code: "PATTERN_MISMATCH", Path: "folder_id"},
			{Value: map[string]any{
				"cloud_id":  "b1glku4lgd6gabcdefgh",
				"folder_id": "b1gia87mbaomkfvsleds",
				"zone":      "ru-central1-c",
				"network":   map[string]any{"kind": "create"},
			}, Code: "CHOICE_NOT_ALLOWED", Path: "zone"},
			{Value: map[string]any{
				"cloud_id":  "b1glku4lgd6gabcdefgh",
				"folder_id": "b1gia87mbaomkfvsleds",
				"zone":      "ru-central1-d",
				"network":   map[string]any{"kind": "existing"},
			}, Code: "REQUIRED_MISSING", Path: "network.network_id"},
			{Value: map[string]any{
				"zone":    "ru-central1-d",
				"network": map[string]any{"kind": "create"},
			}, Code: "REQUIRED_MISSING", Path: "cloud_id"},
		},
	})
}
