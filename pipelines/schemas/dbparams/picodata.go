package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Picodata is db.picodata.params@1 — a Picodata cluster. Picodata has no
// primary/replica split: the cluster is a set of named tiers, each tier a pool
// of instances that Picodata itself groups into replicasets of
// replication_factor size and spreads the buckets over.
//
// doc: https://docs.picodata.io/picodata/stable/
func Picodata() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("picodata", 1)).
		Descr("Picodata topology and options: version, tiers, sharding, memtx memory.").
		Strict().Coerce().
		Fields(
			tuning(),
			// The 26.x line is what the registry publishes today (26.1.x and
			// 26.2.x images, tagged 2026-09); 25.3 is kept because the apt
			// repository still carries it and older result sets reference it.
			// doc: https://docs.picodata.io/picodata/stable/overview/versioning/
			// doc: https://hub.docker.com/r/picodata/picodata/tags
			schemapb.Choice("version").Title("Picodata version").Group("Engine").
				Desc("Picodata series (YY.MINOR); packages come from download.picodata.io.").
				Opt(schemapb.StrV("26.2"), "26.2").
				Opt(schemapb.StrV("26.1"), "26.1").
				Opt(schemapb.StrV("25.3"), "25.3 (legacy)").
				Default(schemapb.StrV("26.1")).Required(),

			// A tier is one `cluster.tier.<name>` block plus the number of
			// instances stroppy-cloud starts in it.
			// doc: https://docs.picodata.io/picodata/stable/reference/config/
			schemapb.List("tiers",
				schemapb.Object("",
					schemapb.Str("name").Title("Tier name").Group("Tier").
						Desc("Key under cluster.tier; instances join a tier by this name.").
						Pattern(`^[a-z][a-z0-9_]{0,31}$`).Required(),
					schemapb.Int64("instances").Title("Instances").Group("Tier").
						Desc("How many picodata instances stroppy-cloud starts in this tier; must divide evenly into replicasets.").
						Gte(1).Lte(64).Default(1),
					// doc: https://docs.picodata.io/picodata/stable/reference/config/
					schemapb.Int64("replication_factor").Title("Replication factor").Group("Tier").
						Desc("Instances per replicaset in this tier (Picodata default 1).").
						Gte(1).Lte(9).Default(1),
					// doc: https://docs.picodata.io/picodata/stable/reference/config/
					schemapb.Bool("can_vote").Title("Raft voter").Group("Tier").
						Desc("Whether instances of this tier take part in the cluster raft vote (Picodata default true).").
						Default(true),
					// doc: https://docs.picodata.io/picodata/stable/reference/config/
					schemapb.Int64("bucket_count").Title("Bucket count").Group("Tier").
						Desc("Sharding buckets for this tier; unset falls back to cluster.default_bucket_count.").
						Gte(1).Lte(1000000),
					// doc: https://docs.picodata.io/picodata/stable/reference/config/
					schemapb.Choice("replication_mode").Title("Replication mode").Group("Tier").
						Desc("Within a replicaset: async = the leader does not wait for replicas, sync = it does (Picodata default async).").
						Opt(schemapb.StrV("async"), "Asynchronous").
						Opt(schemapb.StrV("sync"), "Synchronous").
						Default(schemapb.StrV("async")),
					// 26.2.1 adds the tier's WAL mode: write = a transaction waits for
					// write(2) to the WAL, fsync = for fsync(2) as well. Unset = fsync
					// under tuning auto (every commit durable, as the other engines
					// run), Picodata's own write under stock. 25.3 and 26.1 have no
					// such key and always write without fsync.
					// doc: https://docs.picodata.io/picodata/26.2/reference/config/#cluster_tier_tier_wal_mode
					schemapb.Choice("wal_mode").Title("WAL mode").Group("Tier").
						Desc("26.2 only: write = a commit waits for the WAL write, fsync = for its fsync too. Empty = fsync under tuning auto, Picodata's own write under stock; 25.3 and 26.1 always write without fsync.").
						Opt(schemapb.StrV("write"), "Write (no fsync)").
						Opt(schemapb.StrV("fsync"), "Fsync every write"),
				).Strict(),
			).Title("Tiers").Group("Topology").
				Desc("Cluster tiers; one of them must be named `default` — the tier an instance lands in when it names none.").
				MinItems(1).MaxItems(8),

			// doc: https://docs.picodata.io/picodata/stable/reference/config/
			schemapb.Int64("default_bucket_count").Title("Default bucket count").Group("Sharding").
				Desc("cluster.default_bucket_count — buckets a tier gets when it sets none of its own (Picodata default 3000).").
				Gte(1).Lte(1000000).Default(3000),

			// Several instances on one machine: memtx runs one transaction
			// thread per instance, so a single instance leaves most of a large
			// machine idle. Each instance gets its own ports (base + k - 1) and
			// directory; failure_domain by machine keeps the replicas of one
			// replicaset on different machines.
			// doc: https://docs.picodata.io/picodata/stable/admin/sizing/
			// doc: https://docs.picodata.io/picodata/26.2/reference/config/#instance_failure_domain
			schemapb.Int64("instances_per_machine").Title("Instances per machine").Group("Topology").
				Desc("Picodata instances started on one machine (one memtx transaction thread, one core each). 0 = auto: one instance per vCPU of the machine under tuning auto, one under stock; each tier instance then becomes a machine of that many instances. Otherwise machines = ceil(instances / this). Replicas of a replicaset never share a machine. Clients reach every instance only through HAProxy.").
				Gte(0).Lte(128).Default(0),

			// memtx.memory is a byte size with K/M/G/T suffixes upstream; the
			// schema takes plain megabytes and the recipe renders the suffix.
			// Upstream default is 64M (minimum 32M) — a smoke-test size. Empty
			// sizes it from the machine under tuning auto (RAM less the OS
			// reserve, system memory and 15% engine overhead, split between the
			// instances of the machine); stock renders the product's 2 GiB.
			// doc: https://docs.picodata.io/picodata/26.2/reference/config/#instance_memtx_memory
			// doc: https://docs.picodata.io/picodata/stable/admin/sizing/
			schemapb.Int64("memtx_memory_mb").Title("memtx memory").Group("Memory").
				Desc("instance.memtx.memory per instance. Empty = sized from the machine's RAM under tuning auto, 2 GB under stock; Picodata's own default of 64 MB only fits a smoke test.").
				Unit("MB").Gte(32).Lte(1048576),

			// doc: https://docs.picodata.io/picodata/26.2/reference/settings/#sql_vdbe_opcode_max
			// The docs leave 0 unexplained; on 26.2.1 ALTER SYSTEM accepts 0 and
			// queries then run without the limit. Picodata's own 45000 stops
			// TPC-C validation at scale 100 (GROUP BY over orders).
			schemapb.Int64("sql_vdbe_opcode_max").Title("SQL instruction limit").Group("SQL runtime").
				Desc("Maximum VDBE instructions per local SQL plan; 0 = no limit. Picodata's own default is 45000, which stops full-scan queries such as workload validation.").
				Gte(0).Lte(1000000000).Default(0),

			// Cluster-wide SQL limits, applied by ALTER SYSTEM once the cluster
			// is up. Empty = auto under tuning auto, Picodata's own default under
			// stock.
			// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_runtime_concurrency_max
			// The 25.3 reference has no such setting; the recipe skips it there.
			schemapb.Int64("sql_runtime_concurrency_max").Title("Concurrent SQL queries").Group("SQL runtime").
				Desc("Queries one instance runs at once (Picodata default 50; 26.1 and later). Empty = 1.5 × peak virtual users per SQL endpoint, at least 50, under tuning auto.").
				Gte(1).Lte(100000),
			// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_storage_cache_count_max
			schemapb.Int64("sql_storage_cache_count_max").Title("Storage plan cache entries").Group("SQL runtime").
				Desc("Prepared statements in the LRU cache of a storage instance (Picodata default 50). Empty = 200 under tuning auto.").
				Gte(1).Lte(100000),
			// sql_storage_cache_size_max is in bytes upstream; the schema takes
			// megabytes and the recipe multiplies.
			// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_storage_cache_size_max
			schemapb.Int64("sql_storage_cache_size_max_mb").Title("Storage plan cache size").Group("SQL runtime").
				Desc("sql_storage_cache_size_max, the LRU cache of prepared statements on a storage instance (Picodata default 5 MB). Empty = 32 MB under tuning auto.").
				Unit("MB").Gte(1).Lte(4096),
			// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_motion_row_max
			schemapb.Int64("sql_motion_row_max").Title("Rows per motion").Group("SQL runtime").
				Desc("Rows a virtual table of one query may collect from the storages (Picodata default 5000). Empty = 50000 under tuning auto, so large validation and batch queries do not stop at the limit.").
				Gte(1).Lte(100000000),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances spreading pgproto clients over the instances of the default tier.").
				Gte(0).Lte(2).Default(0),

			// doc: https://docs.picodata.io/picodata/stable/tutorial/connecting/
			schemapb.Bool("pgproto").Title("PostgreSQL protocol").Group("Routing").
				Desc("Expose the PostgreSQL wire-protocol listener; stroppy drives Picodata through it.").
				Default(true),
		).
		Rules(
			schemapb.Rule(`root.tiers.exists(t, t.name == "default")`,
				"one tier must be named `default`").ID("default-tier-required"),
			schemapb.Rule(`root.tiers.all(t, size(root.tiers.filter(u, u.name == t.name)) == 1)`,
				"tier names must be unique").ID("tier-names-unique"),
			schemapb.Rule(`root.tiers.all(t, int(t.instances) % int(t.replication_factor) == 0)`,
				"a tier's instances must divide evenly into replicasets of replication_factor").ID("tier-replicasets-whole"),
			schemapb.Rule(`root.tiers.exists(t, t.can_vote)`,
				"at least one tier must be able to vote, otherwise the cluster raft never elects a leader").ID("some-tier-votes"),
			schemapb.Rule(`root.version == "26.2" || root.tiers.all(t, t.replication_mode == "async")`,
				"synchronous tier replication requires Picodata 26.2").ID("sync-requires-26-2"),
			schemapb.Rule(`root.version == "26.2" || root.tiers.all(t, !("wal_mode" in t) || t.wal_mode == "write")`,
				"the fsync WAL mode requires Picodata 26.2").ID("fsync-requires-26-2"),
			schemapb.Rule(`int(root.haproxy) == 0 || root.pgproto`,
				"HAProxy fronts the pgproto listener, so pgproto must be on").ID("haproxy-needs-pgproto"),
		).
		MustBuild()
}
