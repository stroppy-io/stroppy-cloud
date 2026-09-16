// Package provider holds the schemas of a tenant provider profile: the
// non-secret settings shown in the UI and the credentials that only ever
// travel to a Graphene secret.
package provider

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// ycIDPattern is the shape of a Yandex Cloud resource id. The docs publish no
// formal grammar; every documented example is 20 lowercase alphanumerics
// (clouds and folders start with "b1", service accounts with "aje"), so the
// prefix is treated as a hint and only the shape is enforced.
// doc: https://yandex.cloud/en/docs/resource-manager/operations/folder/get-id
const ycIDPattern = `^[a-z0-9]{20}$`

// YandexSettings is provider.yandex.settings@1 — everything about a Yandex
// Cloud target that is safe to store and show. Credentials live in
// provider.yandex.credentials@1.
func YandexSettings() *schemapb.Schema {
	return schemapb.NewSchema(ids.Provider("yandex", "settings", 1)).
		Descr("Yandex Cloud placement settings of a tenant provider profile (no secrets).").
		Strict().Coerce().
		Fields(
			// doc: https://yandex.cloud/en/docs/resource-manager/operations/folder/get-id
			schemapb.Str("cloud_id").Title("Cloud id").Group("Account").
				Desc("Yandex Cloud cloud id owning the folder.").
				Pattern(ycIDPattern).Required().Examples(schemapb.StrV("b1glku4lgd6gabcdefgh")),
			schemapb.Str("folder_id").Title("Folder id").Group("Account").
				Desc("Folder every VM, disk and network of a run is created in.").
				Pattern(ycIDPattern).Required().Examples(schemapb.StrV("b1gia87mbaomkfvsleds")),

			// doc: https://yandex.cloud/en/docs/overview/concepts/geo-scope —
			// ru-central1-c was decommissioned and no longer exists; -e is the
			// newest Compute zone.
			schemapb.Choice("zone").Title("Zone").Group("Placement").
				Desc("Availability zone of ru-central1 the run is placed in.").
				Opt(schemapb.StrV("ru-central1-a"), "ru-central1-a").
				Opt(schemapb.StrV("ru-central1-b"), "ru-central1-b").
				Opt(schemapb.StrV("ru-central1-d"), "ru-central1-d (recommended)").
				Opt(schemapb.StrV("ru-central1-e"), "ru-central1-e").
				Default(schemapb.StrV("ru-central1-d")).Required(),

			// doc: https://yandex.cloud/en/docs/overview/concepts/geo-scope
			schemapb.List("zones", schemapb.Str("").Pattern(`^ru-central1-[abde]$`)).Title("Distributed topology zones").Group("Placement").
				Desc("Three physical zones for multi-zone topologies. When omitted, use zone and two other catalog zones. Single-zone topologies use zone.").MinItems(3).MaxItems(3).Unique(),

			// doc: https://yandex.cloud/en/docs/compute/concepts/vm-platforms
			schemapb.Choice("platform_id").Title("Platform").Group("Placement").
				Desc("Compute platform (CPU generation) the machines are created on.").
				Opt(schemapb.StrV("standard-v1"), "Intel Broadwell (standard-v1)").
				Opt(schemapb.StrV("standard-v2"), "Intel Cascade Lake (standard-v2)").
				Opt(schemapb.StrV("standard-v3"), "Intel Ice Lake (standard-v3)").
				Opt(schemapb.StrV("standard-v4a"), "AMD Zen 4 (standard-v4a)").
				Opt(schemapb.StrV("amd-v1"), "AMD Zen 3 (amd-v1)").
				Opt(schemapb.StrV("highfreq-v3"), "Intel Ice Lake compute-optimized (highfreq-v3)").
				Opt(schemapb.StrV("highfreq-v4a"), "AMD Zen 4 compute-optimized (highfreq-v4a)").
				Default(schemapb.StrV("standard-v3")),

			schemapb.OneOf("network", "kind").Title("Network").Group("Network").
				Desc("Create a throwaway network for every run, or place runs into an existing one.").
				Variant("create",
					schemapb.Str("subnet_cidr").Title("Subnet CIDR").
						Desc("IPv4 range of the subnet created for the run.").
						Pattern(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`).Default("10.130.0.0/24"),
				).
				Variant("existing",
					schemapb.Str("network_id").Title("Network id").
						Desc("Existing VPC network the machines join.").
						Pattern(ycIDPattern).Required(),
					schemapb.Str("subnet_id").Title("Subnet id").
						Desc("Existing subnet in the chosen zone.").
						Pattern(ycIDPattern).Required(),
					schemapb.Str("security_group_id").Title("Security group id").
						Desc("Existing security group applied to every machine.").
						Pattern(ycIDPattern),
				).
				Required(),

			schemapb.Bool("public_ips").Title("Public IPs").Group("Network").
				Desc("Assign a one-to-one NAT public address to every machine.").Default(true),

			// doc: https://yandex.cloud/en/docs/compute/operations/images-with-pre-installed-software/get-list
			// (families live in the standard-images folder; centos-stream-9 is
			// published as centos-stream-9-oslogin).
			schemapb.Choice("image_family").Title("Image family").Group("Placement").
				Desc("Boot image family from the standard-images folder.").
				Opt(schemapb.StrV("ubuntu-2404-lts"), "Ubuntu 24.04 LTS").
				Opt(schemapb.StrV("ubuntu-2204-lts"), "Ubuntu 22.04 LTS").
				Opt(schemapb.StrV("ubuntu-2004-lts"), "Ubuntu 20.04 LTS").
				Opt(schemapb.StrV("debian-12"), "Debian 12").
				Opt(schemapb.StrV("centos-stream-9-oslogin"), "CentOS Stream 9").
				Default(schemapb.StrV("ubuntu-2404-lts")),

			// doc: https://yandex.cloud/en/docs/compute/concepts/preemptible-vm —
			// a preemptible VM is stopped after 24 hours or on resource pressure.
			schemapb.Bool("preemptible").Title("Preemptible").Group("Placement").
				Desc("Use preemptible VMs: much cheaper, but stopped after 24h or under pressure — "+
					"do not use for a measurement that must complete.").
				Default(false),
		).
		MustBuild()
}
