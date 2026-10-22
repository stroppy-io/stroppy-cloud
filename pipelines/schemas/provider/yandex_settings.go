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

// cidrPoolPattern is an IPv4 range of /8 to /24: wide enough for at least
// one /24 run block, narrow enough to stay inside one RFC 1918 range.
const cidrPoolPattern = `^(\d{1,3}\.){3}\d{1,3}/([89]|1\d|2[0-4])$`

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
				Desc("Three physical zones for multi-zone topologies. When omitted, use zone and two other catalog zones. Single-zone topologies use zone.").MaxItems(3).Unique().
				// An empty list is the omitted one: a form sends [] for an untouched list.
				Rules(schemapb.Rule(`size(this) == 0 || size(this) == 3`, "give three zones or none").ID("zones-none-or-three")),

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

			// One network per profile: stroppy-provider-config creates it with
			// the profile (create) or the user names one (existing); every run
			// cuts its zonal subnets out of pool_cidr. Subnets take /28 to /16
			// of an RFC 1918 range; the pool is the range the server allocates
			// run blocks (/28../24) from.
			// doc: https://yandex.cloud/en/docs/vpc/concepts/network#subnet
			schemapb.OneOf("network", "kind").Title("Network").Group("Network").
				Desc("Create one network for the profile, or place runs into an existing one. Every run gets its own subnets and security group in it.").
				Variant("create",
					schemapb.Str("pool_cidr").Title("Address pool").
						Desc("IPv4 range the run subnets are allocated from, one block per zone sized to its machines.").
						Pattern(cidrPoolPattern).Default("10.128.0.0/16"),
					// Retired: a run no longer owns a whole network, so one fixed
					// range would collide between runs. The field stays declared
					// (and never active) so stored profiles still load under Strict.
					schemapb.Str("subnet_cidr").Title("Subnet CIDR").
						Desc("Retired: run subnets come from pool_cidr; a stored value is ignored.").
						Deprecated().When("false"),
				).
				Variant("existing",
					schemapb.Str("network_id").Title("Network id").
						Desc("Existing VPC network the run subnets are created in.").
						Pattern(ycIDPattern).Required(),
					schemapb.Str("pool_cidr").Title("Address pool").
						Desc("IPv4 range of the network kept free for runs; run subnets are allocated from it and must not overlap other subnets of the network.").
						Pattern(cidrPoolPattern).Required(),
					// doc: https://yandex.cloud/en/docs/vpc/concepts/routing
					schemapb.Str("route_table_id").Title("Route table id").
						Desc("Route table with a route out (e.g. 0.0.0.0/0 through a NAT gateway) for machines without public addresses. When empty, such a run creates its own NAT gateway.").
						Pattern(ycIDPattern),
				).
				Required(),

			// doc: https://yandex.cloud/en/docs/vpc/concepts/gateways
			schemapb.Bool("public_ips").Title("Public IPs").Group("Network").
				Desc("Assign a one-to-one NAT public address to every machine. Without them a created network gets one shared NAT gateway for the profile.").Default(true),

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
