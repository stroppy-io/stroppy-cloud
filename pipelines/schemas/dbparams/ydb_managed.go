package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// YdbManaged is db.ydb_managed.params@1 — Yandex Managed Service for YDB. No
// machines are provisioned for it: terraform creates either a
// yandex_ydb_database_serverless or a yandex_ydb_database_dedicated and the
// runner talks to the managed endpoint.
//
// doc: https://yandex.cloud/en/docs/ydb/concepts/serverless-and-dedicated
func YdbManaged() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("ydb_managed", 1)).
		Descr("Yandex Managed YDB: serverless or dedicated database, its presets, scaling and storage.").
		Strict().Coerce().
		Fields(
			// The two modes are two different terraform resources and the mode
			// cannot be changed after creation.
			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_serverless
			schemapb.Choice("type").Title("Database type").Group("Engine").
				Desc("serverless = pay-per-request, YC-managed endpoint, no VPC; dedicated = a sized cluster in your network.").
				Opt(schemapb.StrV("dedicated"), "Dedicated").
				Opt(schemapb.StrV("serverless"), "Serverless").
				Default(schemapb.StrV("dedicated")).Required(),

			// doc: https://yandex.cloud/en/docs/overview/concepts/geo-scope
			schemapb.Str("location_id").Title("Availability zone group").Group("Engine").
				Desc("YC location the database is created in (location_id on both resources).").
				Pattern(`^[a-z0-9\-]+$`).MaxLen(32).Default("ru-central1"),

			// Preset table as published; YC's own docs say the authoritative
			// list is `yc ydb resource-preset list`, so the choice stays open.
			// doc: https://yandex.cloud/en/docs/ydb/concepts/resources
			schemapb.Choice("resource_preset_id").Title("Resource preset").Group("Dedicated").
				Desc("VM configuration of one database node: vCPUs and RAM.").
				Opt(schemapb.StrV("medium"), "medium — 8 vCPU / 32 GB").
				Opt(schemapb.StrV("medium-m64"), "medium-m64 — 8 vCPU / 64 GB").
				Opt(schemapb.StrV("medium-m96"), "medium-m96 — 8 vCPU / 96 GB").
				Opt(schemapb.StrV("large"), "large — 12 vCPU / 48 GB").
				Opt(schemapb.StrV("xlarge"), "xlarge — 16 vCPU / 64 GB").
				Opt(schemapb.StrV("oltp-c16-m128"), "oltp-c16-m128 — 16 vCPU / 128 GB").
				Opt(schemapb.StrV("olap-c16-m128"), "olap-c16-m128 — 16 vCPU / 128 GB").
				Default(schemapb.StrV("medium")).Open().
				When(`root.type == "dedicated"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			schemapb.Choice("scale_policy").Title("Scale policy").Group("Dedicated").
				Desc("fixed = a constant number of nodes; auto = YC scales between min and max on CPU utilization (a preview feature, needs the enable_autoscaling label).").
				Opt(schemapb.StrV("fixed"), "Fixed size").
				Opt(schemapb.StrV("auto"), "Auto scale (preview)").
				Default(schemapb.StrV("fixed")).
				When(`root.type == "dedicated"`),

			// scale_policy.fixed_scale.size; YC recommends >= 3 slots for HA.
			// doc: https://yandex.cloud/en/docs/ydb/operations/manage-databases
			schemapb.Int64("node_count").Title("Nodes").Group("Dedicated").
				Desc("scale_policy.fixed_scale.size — database slots; production guidance is at least three.").
				Gte(1).Lte(64).Default(3).
				When(`root.type == "dedicated" && root.scale_policy == "fixed"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			schemapb.Object("auto_scale",
				schemapb.Int64("min_size").Title("Min nodes").Group("Dedicated").
					Desc("scale_policy.auto_scale.min_size.").Gte(1).Lte(64).Default(2),
				schemapb.Int64("max_size").Title("Max nodes").Group("Dedicated").
					Desc("scale_policy.auto_scale.max_size.").Gte(1).Lte(64).Default(8),
				schemapb.Int64("cpu_utilization_percent").Title("Target CPU").Group("Dedicated").
					Desc("scale_policy.auto_scale.target_tracking.cpu_utilization_percent — the utilization YC scales to hold.").
					Unit("%").Gte(10).Lte(100).Default(70),
			).Strict().Title("Auto scale").Group("Dedicated").
				Desc("Target-tracking autoscaling bounds; only read when scale_policy is auto.").
				When(`root.type == "dedicated" && root.scale_policy == "auto"`),

			// storage_config.group_count; for ssd one group holds up to 100 GB.
			// doc: https://yandex.cloud/en/docs/ydb/operations/manage-databases
			schemapb.Int64("storage_groups").Title("Storage groups").Group("Dedicated").
				Desc("storage_config.group_count — for the ssd type one group stores up to 100 GB, so size this from the dataset.").
				Gte(1).Lte(1024).Default(1).
				When(`root.type == "dedicated"`),

			// Only `ssd` is documented by example; the authoritative list is
			// `yc ydb storage-type list`, hence Open.
			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			schemapb.Choice("storage_type").Title("Storage type").Group("Dedicated").
				Desc("storage_config.storage_type_id; YC documents ssd by example and defers the full list to `yc ydb storage-type list`, so other ids are accepted.").
				Opt(schemapb.StrV("ssd"), "ssd").
				Default(schemapb.StrV("ssd")).Open().
				When(`root.type == "dedicated"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			schemapb.Bool("assign_public_ips").Title("Public IPs").Group("Dedicated").
				Desc("Give the dedicated database nodes public addresses; only meaningful for a dedicated database (serverless has no VPC attachment).").
				Default(false).
				When(`root.type == "dedicated"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_serverless
			schemapb.Int64("throttling_rcu_limit").Title("Throttling RCU limit").Group("Serverless").
				Desc("serverless_database.throttling_rcu_limit — request units per second ceiling; 0 disables throttling.").
				Unit("RU/s").Gte(0).Lte(1000000).Default(0).
				When(`root.type == "serverless"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_serverless
			schemapb.Int64("provisioned_rcu_limit").Title("Provisioned RCU limit").Group("Serverless").
				Desc("serverless_database.provisioned_rcu_limit — reserved RU/s billed hourly; 0 disables provisioned capacity.").
				Unit("RU/s").Gte(0).Lte(1000000).Default(0).
				When(`root.type == "serverless"`),

			// doc: https://yandex.cloud/en/docs/ydb/operations/manage-databases
			schemapb.Int64("storage_size_limit_gb").Title("Storage limit").Group("Serverless").
				Desc("serverless_database.storage_size_limit — the database's data ceiling (YC default 50 GB).").
				Unit("GB").Gte(1).Lte(10000).Default(50).
				When(`root.type == "serverless"`),

			// doc: https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/ydb_database_dedicated
			schemapb.Bool("deletion_protection").Title("Deletion protection").Group("Lifecycle").
				Desc("Refuse terraform destroy on the database; a kept stand still tears down its VMs, so leave this off unless the data matters.").
				Default(false),
		).
		Rules(
			// OLTP vs OLAP is not a database-level knob in the YC API or the
			// terraform provider: row store vs column store is chosen per table
			// (`store = "column"`), so this schema deliberately has no
			// compute_type field.
			// doc: https://yandex.cloud/en/docs/ydb/terraform/tables
			schemapb.Rule(`!("auto_scale" in root) || int(root.auto_scale.min_size) <= int(root.auto_scale.max_size)`,
				"auto_scale.min_size must not exceed auto_scale.max_size").ID("autoscale-min-le-max"),
			schemapb.Rule(`!("node_count" in root) || int(root.node_count) >= 3`,
				"a dedicated database with fewer than three slots has no redundancy").ID("dedicated-ha").Warn(),
			schemapb.Rule(`!("storage_groups" in root) || root.storage_type != "ssd" || int(root.storage_groups) >= 2`,
				"one ssd storage group holds about 100 GB — a larger dataset needs more groups").ID("ssd-group-capacity").Warn(),
		).
		MustBuild()
}
