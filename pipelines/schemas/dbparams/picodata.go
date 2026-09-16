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
				).Strict(),
			).Title("Tiers").Group("Topology").
				Desc("Cluster tiers; one of them must be named `default` — the tier an instance lands in when it names none.").
				MinItems(1).MaxItems(8),

			// doc: https://docs.picodata.io/picodata/stable/reference/config/
			schemapb.Int64("default_bucket_count").Title("Default bucket count").Group("Sharding").
				Desc("cluster.default_bucket_count — buckets a tier gets when it sets none of its own (Picodata default 3000).").
				Gte(1).Lte(1000000).Default(3000),

			// memtx.memory is a byte size with K/M/G/T suffixes upstream; the
			// schema takes plain megabytes and the recipe renders the suffix.
			// Upstream default is 64M (minimum 32M) — a smoke-test size, so the
			// product default is 2 GiB.
			// doc: https://docs.picodata.io/picodata/stable/reference/config/
			schemapb.Int64("memtx_memory_mb").Title("memtx memory").Group("Memory").
				Desc("instance.memtx.memory per instance; Picodata's own default of 64 MB only fits a smoke test.").
				Unit("MB").Gte(32).Lte(1048576).Default(2048),

			// doc: https://docs.picodata.io/picodata/26.2/reference/settings/#sql_vdbe_opcode_max
			schemapb.Int64("sql_vdbe_opcode_max").Title("SQL instruction limit").Group("SQL runtime").
				Desc("Maximum VDBE instructions per local SQL plan. Picodata defaults to 45000; full-scan workload validation can require a higher explicit limit.").
				Gte(1).Lte(1000000000).Default(45000),

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
			schemapb.Rule(`int(root.haproxy) == 0 || root.pgproto`,
				"HAProxy fronts the pgproto listener, so pgproto must be on").ID("haproxy-needs-pgproto"),
		).
		MustBuild()
}
