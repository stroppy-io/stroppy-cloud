package provider

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestAwsSettings(t *testing.T) {
	minimal := map[string]any{
		"region":  "eu-central-1",
		"network": map[string]any{"kind": "create"},
	}
	full := map[string]any{
		"region":            "us-east-2",
		"availability_zone": "us-east-2b",
		"network": map[string]any{
			"kind":              "existing",
			"vpc_id":            "vpc-0123456789abcdef0",
			"subnet_id":         "subnet-0123456789abcdef0",
			"security_group_id": "sg-0123456789abcdef0",
		},
		"instance_family": "r7i",
		"ami_family":      "al2023",
		"public_ips":      false,
		"spot":            true,
	}

	schematest.Run(t, AwsSettings(), schematest.Cases{
		Valid: []map[string]any{minimal},
		Invalid: []schematest.Invalid{
			{Value: full, Code: "RULE_VIOLATED", Path: "network-create-only"},
			{Value: map[string]any{
				"region":  "us-gov-west-1",
				"network": map[string]any{"kind": "create"},
			}, Code: "CHOICE_NOT_ALLOWED", Path: "region"},
			{Value: map[string]any{
				"region":            "eu-central-1",
				"availability_zone": "eu-central-1",
				"network":           map[string]any{"kind": "create"},
			}, Code: "PATTERN_MISMATCH", Path: "availability_zone"},
			{Value: map[string]any{
				"region":  "eu-central-1",
				"network": map[string]any{"kind": "existing", "vpc_id": "vpc-zzz", "subnet_id": "subnet-0123456789abcdef0"},
			}, Code: "PATTERN_MISMATCH", Path: "network.vpc_id"},
			{Value: map[string]any{
				"region":  "eu-central-1",
				"network": map[string]any{"kind": "byo"},
			}, Code: "UNKNOWN_VARIANT", Path: "network"},
		},
	})
}
