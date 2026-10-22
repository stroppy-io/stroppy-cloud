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

			// Kept so params saved before it was dropped still bake; hidden
			// and ignored. The stand puts one fail domain on every storage
			// node (rack = node, static group one VDisk per node), and the
			// config's mirror-3-dc / block-4-2 rules already want 9 / 8 hosts,
			// so a disk-level domain never reached the cluster. Disk-level
			// domains need their own pool geometry and a live check.
			// doc: https://ydb.tech/docs/en/concepts/topology
			schemapb.Str("failure_domain").Title("Fail domain level").Group("Storage").
				Desc("Removed: every storage node is one fail domain. Ignored when present.").
				MaxLen(16).When("false").Deprecated(),

			// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
			schemapb.Int64("storage_nodes").Title("Storage nodes").Group("Topology").
				Desc("Nodes running the BlobStorage layer; together with pdisks_per_node they supply the fail domains the erasure mode demands.").
				Gte(1).Lte(64).Default(1),

			// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
			schemapb.Int64("database_nodes").Title("Database nodes").Group("Topology").
				Desc("Nodes running the query/tablet layer of the database; these are what the workload connects to.").
				Gte(1).Lte(64).Default(1),

			// The stand puts pdisks in files under /data/ydb/pdisk-N.data on
			// the data disk, so the count is a knob rather than a property of
			// the machine. Files on one disk share its IOPS: more pdisks give
			// the storage groups more slots, not more I/O.
			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/block-4-2.yaml
			schemapb.Int64("pdisks_per_node").Title("PDisks per storage node").Group("Storage").
				Desc("Drive entries per host_config, each a file of at least 80 GB on the node's data disk (the disk needs 80 GB per pdisk + 20 GB).").
				Gte(1).Lte(8).Default(1),

			// The form spells the media SSD, NVME and HDD; the recipe writes
			// the config's SSD / NVME / ROT (host_configs drive.type, pool
			// pdisk filter, static group pdisk_category).
			// doc: https://ydb.tech/docs/en/reference/configuration/host_configs
			// doc: https://github.com/ydb-platform/ydb/blob/main/ydb/deploy/yaml_config_examples/block-4-2.yaml
			schemapb.Choice("disk_type").Title("PDisk device type").Group("Storage").
				Desc("Drive type of every host_config entry, the storage pool's pdisk filter and pool kind (ssd / nvme / rot) the database is created in.").
				Opt(schemapb.StrV("SSD"), "SSD").
				Opt(schemapb.StrV("NVME"), "NVMe").
				Opt(schemapb.StrV("HDD"), "HDD (rotational)").
				Default(schemapb.StrV("SSD")).Required(),

			// No default: unset is sized by the tuning (auto: 4 for erasure
			// none, 2 per 4 database-node cores clamped to 6..24 for
			// mirror-3-dc, within 8 VDisks per pdisk; stock: 1).
			// doc: https://ydb.tech/docs/en/concepts/topology
			schemapb.Int64("storage_groups").Title("Storage groups").Group("Storage").
				Desc("Blob storage groups created in the database's storage pool; more groups spread tablets and their logs wider. Empty = sized by the tuning (auto: 4, mirror-3-dc from the database-node cores; stock: 1).").
				Gte(1).Lte(1024).Nullable(),

			// 80 GB is the smallest device YDB uses.
			// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
			schemapb.Bool("auto_size_pdisks").Title("Auto-size pdisks").Group("Storage").
				Desc("Size each pdisk file from the data disk: 80% of (disk - 10 GB) split over the node's pdisks, at least 80 GB. Off = 80 GB each.").
				Default(true),

			// /Root is the cluster root; a database is a directory under it.
			// doc: https://ydb.tech/docs/en/reference/ydb-cli/commands/dir
			schemapb.Str("database_path").Title("Database path").Group("Database").
				Desc("Full path of the benchmark database inside the cluster; /Root itself is reserved for the cluster root.").
				Pattern(`^/Root/[A-Za-z0-9_][A-Za-z0-9_\-]*(/[A-Za-z0-9_][A-Za-z0-9_\-]*)*$`).
				MaxLen(255).Default("/Root/stroppy"),

			// Kept so params saved before it was dropped still bake; hidden
			// and ignored. The stand issues no certificates and serves plain
			// grpc:// only; the field returns with certificate issuing.
			// doc: https://ydb.tech/docs/en/concepts/connect
			schemapb.Bool("grpcs").Title("TLS (grpcs)").Group("Database").
				Desc("Removed until the stand issues certificates: the endpoint is plain grpc://. Ignored when present.").
				When("false").Deprecated(),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances spreading gRPC clients over the database nodes.").
				Gte(0).Lte(2).Default(0),

			schemapb.Computed("fail_domains", `int(root.storage_nodes)`).
				Result(schemapb.ResultInt64).Title("Fail domains").Group("Storage").
				Desc("How many independent failure units the storage layer gets: one per storage node — what the erasure mode is checked against."),
		).
		Rules(
			schemapb.Rule(`root.fault_tolerance != "block-4-2" || int(root.storage_nodes) >= 8`,
				"block-4-2 needs at least 8 fail domains (one per storage node)").ID("block42-min-domains"),
			schemapb.Rule(`root.fault_tolerance != "mirror-3-dc" || int(root.storage_nodes) >= 9`,
				"mirror-3-dc needs at least 9 fail domains (storage nodes) across 3 realms").ID("mirror3dc-min-domains"),
			schemapb.Rule(`root.fault_tolerance != "mirror-3-dc" || int(root.storage_nodes) % 3 == 0`,
				"mirror-3-dc spreads hosts over exactly 3 datacenters, so storage_nodes must be a multiple of 3").ID("mirror3dc-three-realms"),
			schemapb.Rule(`root.fault_tolerance != "none" || int(root.storage_nodes) >= 1`,
				"erasure none still needs one storage node").ID("none-min-domains"),
			schemapb.Rule(`int(root.haproxy) == 0 || int(root.database_nodes) >= 2`,
				"HAProxy in front of a single database node only adds a hop").ID("haproxy-pointless").Warn(),
		).
		MustBuild()
}
