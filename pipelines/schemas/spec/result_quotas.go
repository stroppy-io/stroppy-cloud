package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// ResultQuotas is spec.result.quotas@1 — the quota snapshot the stroppy-quotas
// pipeline returns; it fills the tenant quota screen and gates launches.
//
// The JSON shape matches OpenAPI QuotaReport
// (openapi/parts/30-tenant-settings.yaml).
//
// doc: quota ids — https://yandex.cloud/en/docs/compute/concepts/limits
// (compute.instances.count, compute.instanceCores.count,
// compute.instanceMemory.size, compute.ssdDisks.size, compute.hddDisks.size);
// AWS uses Service Quotas codes (L-…) under the ec2 service.
func ResultQuotas() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("result.quotas", 1)).
		Descr("Provider quota snapshot: limits and current usage, as observed at a point in time.").
		Strict().Coerce().
		Fields(
			schemapb.Str("unavailable_reason").Title("Unavailable reason").Group("Result").
				Desc("permission_denied when cloud quotas cannot be read. Quotas are empty and launches rely on provider enforcement instead of the cloud precheck.").MaxLen(64),
			schemapb.Str("scope").Title("Scope").Group("Result").
				Desc("Scope of the quota snapshot or denied request, e.g. cloud:b1g... for Yandex Cloud. Cloud quotas cover all folders and zones.").MaxLen(256),
			schemapb.Timestamp("observed_at").Title("Observed at").Group("Result").
				Desc("When the provider was read; the server treats a snapshot older than a minute as stale.").
				Required(),
			schemapb.List("quotas",
				schemapb.Object("",
					schemapb.Str("name").Title("Quota").
						Desc("Provider quota id, e.g. compute.instanceCores.count.").
						MinLen(1).MaxLen(128).Required(),
					schemapb.Double("limit").Title("Limit").
						Desc("Upper bound the provider enforces.").Gte(0).Required(),
					schemapb.Double("used").Title("Used").
						Desc("Consumption at observed_at across the quota scope: all folders in the cloud for Yandex Cloud, or the AWS account.").
						Gte(0).Required(),
					schemapb.Str("unit").Title("Unit").
						Desc("Unit of limit and used: count, GB, GiB…").MaxLen(32),
					schemapb.Str("zone").Title("Zone").
						Desc("Set for a zone-scoped quota; empty when the quota is regional or global.").
						MaxLen(64),
				).Strict(),
			).Title("Quotas").Group("Result").
				Desc("One entry per quota the provider reports for this profile.").
				MaxItems(512).Required(),
		).
		MustBuild()
}
