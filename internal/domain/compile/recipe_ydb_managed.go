package compile

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func managedYDBRecipe(c *compilation) error {
	p := c.params()
	if c.in.ProviderKind != "yandex" {
		return errs.Newf(errs.CodeInvalid, "Managed YDB requires Yandex Cloud")
	}
	if c.in.CredentialsSecret == "" {
		return errs.Newf(errs.CodeInvalid, "Managed YDB requires named YC credentials for IAM authentication")
	}
	if boolParam(p, "deletion_protection") {
		return errs.Newf(errs.CodeInvalid, "Managed YDB run resources require deletion_protection=false so cleanup can remove them")
	}
	m := &spec.ManagedYDB{Type: strParam(p, "type", "dedicated"), LocationID: strParam(p, "location_id", "ru-central1")}
	switch m.Type {
	case "dedicated":
		if strParam(p, "scale_policy", "fixed") != "fixed" {
			return errs.Newf(errs.CodeInvalid, "Managed YDB autoscaling is unsupported by the installed Crossplane provider")
		}
		m.ResourcePresetID = strParam(p, "resource_preset_id", "medium")
		m.NodeCount = intParam(p, "node_count", 3)
		m.StorageGroups = intParam(p, "storage_groups", 1)
		m.StorageType = strParam(p, "storage_type", "ssd")
		m.AssignPublicIPs = boolParam(p, "assign_public_ips")
		var settings map[string]any
		if err := json.Unmarshal(c.in.ProviderSettings, &settings); err != nil {
			return err
		}
		var err error
		m.Zones, err = c.threeYandexZones(settings)
		if err != nil {
			return err
		}
	case "serverless":
		m.ThrottlingRCULimit = intParam(p, "throttling_rcu_limit", 0)
		m.ProvisionedRCULimit = intParam(p, "provisioned_rcu_limit", 0)
		m.StorageSizeLimitGB = intParam(p, "storage_size_limit_gb", 50)
	default:
		return errs.Newf(errs.CodeInvalid, "unknown Managed YDB type %q", m.Type)
	}
	c.out.Spec.ManagedYDB = m
	return nil
}
