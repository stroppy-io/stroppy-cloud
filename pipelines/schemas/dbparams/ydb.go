package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Ydb is db.ydb.params@1 — a self-hosted YDB cluster. YDB splits the cluster
// in two node classes: storage nodes carry the distributed BlobStorage layer,
// database nodes run the query/tablet layer over it. The erasure mode picks
// how many independent fail domains the storage layer needs, and that is what
// actually sizes the stand.
//
// doc: https://ydb.tech/docs/en/concepts/topology
func Ydb() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("ydb", 1)).
		Descr("Self-hosted YDB topology: storage/database nodes, erasure mode, pdisks, database path.").
		Strict().Coerce().
		Fields(
			tuning(),
			// Server lines published on the registry as of 2026-09-08: 26.3
			// (latest), 26.2, 26.1, 25.4. 24.x is no longer built.
			// doc: https://ydb.tech/docs/en/changelog-server
			// doc: https://hub.docker.com/r/ydbplatform/local-ydb/tags
			schemapb.Choice("version").Title("YDB version").Group("Engine").
				Desc("ydbd server series; binaries are fetched through the stroppy gateway.").
				Opt(schemapb.StrV("26.3"), "26.3 (latest)").
				Opt(schemapb.StrV("26.2"), "26.2").
				Opt(schemapb.StrV("26.1"), "26.1").
				Opt(schemapb.StrV("25.4"), "25.4").
				Default(schemapb.StrV("26.2")).Required(),

			// `erasure` in the MainConfig; the literal strings are the config
			// values, not our own spelling.
			// doc: https://ydb.tech/docs/en/concepts/topology
			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/block-4-2.yaml
			schemapb.Choice("fault_tolerance").Title("Erasure mode").Group("Storage").
				Desc("config.erasure: none = no redundancy (dev only); block-4-2 = 4 data + 2 parity over >=8 fail domains; mirror-3-dc = 3 realms x 3 domains, >=9 nodes.").
				Opt(schemapb.StrV("none"), "none (no redundancy)").
				Opt(schemapb.StrV("block-4-2"), "block-4-2 (>= 8 fail domains)").
				Opt(schemapb.StrV("mirror-3-dc"), "mirror-3-dc (>= 9 nodes, 3 DCs)").
				Default(schemapb.StrV("none")).Required(),

			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/mirror-3dc-9-nodes.yaml
			schemapb.Choice("failure_domain").Title("Fail domain level").Group("Storage").
				Desc("config.fail_domain_type: which level of host.location the storage layer treats as one failure unit. Upstream uses disk for single-host block-4-2 and rack for mirror-3-dc.").
				Opt(schemapb.StrV("disk"), "disk (all pdisks of a host are independent)").
				Opt(schemapb.StrV("body"), "body (one host = one domain)").
				Opt(schemapb.StrV("rack"), "rack (one rack = one domain)").
				Default(schemapb.StrV("disk")).Required(),

			// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
			schemapb.Int64("storage_nodes").Title("Storage nodes").Group("Topology").
				Desc("Nodes running the BlobStorage layer; together with pdisks_per_node they supply the fail domains the erasure mode demands.").
				Gte(1).Lte(64).Default(1),

			// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
			schemapb.Int64("database_nodes").Title("Database nodes").Group("Topology").
				Desc("Nodes running the query/tablet layer of the database; these are what the workload connects to.").
				Gte(1).Lte(64).Default(1),

			// The stand puts pdisks in files under
			// /var/lib/stroppy-cloud/ydb-pdisk-N.data, so the count is a knob
			// rather than a property of the machine.
			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/block-4-2.yaml
			schemapb.Int64("pdisks_per_node").Title("PDisks per storage node").Group("Storage").
				Desc("Drive entries per host_config; with failure_domain=disk each pdisk is its own fail domain, which is how a small stand reaches 8 or 9 of them.").
				Gte(1).Lte(8).Default(1),

			// The example configs spell the drive kinds SSD, NVME and HDD;
			// the older ROT spelling is gone.
			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/block-4-2.yaml
			schemapb.Choice("disk_type").Title("PDisk device type").Group("Storage").
				Desc("config.default_disk_type and the drive type of every host_config entry; storage pools are built per device type.").
				Opt(schemapb.StrV("SSD"), "SSD").
				Opt(schemapb.StrV("NVME"), "NVMe").
				Opt(schemapb.StrV("HDD"), "HDD (rotational)").
				Default(schemapb.StrV("SSD")).Required(),

			// doc: https://ydb.tech/docs/en/concepts/topology
			schemapb.Int64("storage_groups").Title("Storage groups").Group("Storage").
				Desc("Blob storage groups created in the database's storage pool; more groups spread tablets wider.").
				Gte(1).Lte(1024).Default(1),

			schemapb.Bool("auto_size_pdisks").Title("Auto-size pdisks").Group("Storage").
				Desc("Let the recipe size the pdisk files from the machine's free disk instead of a fixed size.").
				Default(true),

			// /Root is the cluster root; a database is a directory under it.
			// doc: https://ydb.tech/docs/en/reference/ydb-cli/commands/dir
			schemapb.Str("database_path").Title("Database path").Group("Database").
				Desc("Full path of the benchmark database inside the cluster; /Root itself is reserved for the cluster root.").
				Pattern(`^/Root/[A-Za-z0-9_][A-Za-z0-9_\-]*(/[A-Za-z0-9_][A-Za-z0-9_\-]*)*$`).
				MaxLen(255).Default("/Root/stroppy"),

			// doc: https://ydb.tech/docs/en/concepts/connect
			schemapb.Bool("grpcs").Title("TLS (grpcs)").Group("Database").
				Desc("Serve the gRPC endpoint over TLS (grpcs://) with a self-signed cluster certificate instead of plain grpc://.").
				Default(false),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances spreading gRPC clients over the database nodes.").
				Gte(0).Lte(2).Default(0),

			schemapb.Computed("fail_domains", `int(root.storage_nodes) * (root.failure_domain == "disk" ? int(root.pdisks_per_node) : 1)`).
				Result(schemapb.ResultInt64).Title("Fail domains").Group("Storage").
				Desc("How many independent failure units the storage layer actually gets — what the erasure mode is checked against."),
		).
		Rules(
			schemapb.Rule(`root.fault_tolerance != "block-4-2" || int(root.storage_nodes) * (root.failure_domain == "disk" ? int(root.pdisks_per_node) : 1) >= 8`,
				"block-4-2 needs at least 8 fail domains (storage_nodes x pdisks_per_node when failure_domain=disk)").ID("block42-min-domains"),
			schemapb.Rule(`root.fault_tolerance != "mirror-3-dc" || int(root.storage_nodes) * (root.failure_domain == "disk" ? int(root.pdisks_per_node) : 1) >= 9`,
				"mirror-3-dc needs at least 9 fail domains across 3 realms").ID("mirror3dc-min-domains"),
			schemapb.Rule(`root.fault_tolerance != "mirror-3-dc" || int(root.storage_nodes) % 3 == 0`,
				"mirror-3-dc spreads hosts over exactly 3 datacenters, so storage_nodes must be a multiple of 3").ID("mirror3dc-three-realms"),
			schemapb.Rule(`root.fault_tolerance != "none" || int(root.storage_nodes) >= 1`,
				"erasure none still needs one storage node").ID("none-min-domains"),
			schemapb.Rule(`int(root.haproxy) == 0 || int(root.database_nodes) >= 2`,
				"HAProxy in front of a single database node only adds a hop").ID("haproxy-pointless").Warn(),
		).
		MustBuild()
}
