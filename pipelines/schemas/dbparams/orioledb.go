package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Orioledb is db.orioledb.params@1 — PostgreSQL with the OrioleDB storage
// engine, deployed from the upstream container image (there is no apt build).
// OrioleDB is a patched postgres, so the topology is postgres minus Patroni:
// upstream ships no failover tooling for it, only physical async standbys.
//
// doc: https://github.com/orioledb/orioledb
func Orioledb() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("orioledb", 1)).
		Descr("OrioleDB (containerised PostgreSQL storage engine) topology and options.").
		Strict().Coerce().
		Fields(
			tuning(),
			// Tag list verified against the registry on 2026-09-08: the
			// repository publishes latest-pgNN (tracking main), beta17-pgNN
			// (the beta17 release) and pgNN-nightly for majors 16, 17, 18.
			// The project calls itself public beta — not production software,
			// which is fine for a benchmark stand.
			// doc: https://hub.docker.com/r/orioledb/orioledb/tags
			schemapb.Choice("image_tag").Title("Image tag").Group("Engine").
				Desc("Tag of the orioledb/orioledb image; the pgNN suffix picks the PostgreSQL major the engine is patched into.").
				Opt(schemapb.StrV("latest-pg18"), "latest-pg18 (main, PG 18)").
				Opt(schemapb.StrV("latest-pg17"), "latest-pg17 (main, PG 17)").
				Opt(schemapb.StrV("latest-pg16"), "latest-pg16 (main, PG 16)").
				Opt(schemapb.StrV("beta17-pg18"), "beta17-pg18 (release, PG 18)").
				Opt(schemapb.StrV("beta17-pg17"), "beta17-pg17 (release, PG 17)").
				Opt(schemapb.StrV("beta17-pg16"), "beta17-pg16 (release, PG 16)").
				Default(schemapb.StrV("beta17-pg17")).Required(),

			// doc: https://hub.docker.com/r/orioledb/orioledb/tags
			schemapb.Bool("ubuntu_base").Title("Ubuntu base image").Group("Engine").
				Desc("Use the -ubuntu flavor of the tag instead of the default Alpine one; needed when the workload wants glibc locales.").
				Default(false),

			// OrioleDB rides PostgreSQL physical replication; upstream has no
			// Patroni integration, so failover is manual by construction.
			// doc: https://www.postgresql.org/docs/current/warm-standby.html
			schemapb.Int64("replicas").Title("Streaming replicas").Group("Topology").
				Desc("Physical async standbys; OrioleDB has no supported failover manager, so these never get promoted automatically.").
				Gte(0).Lte(8).Default(0),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances in front of the master (and read-only backends for the replicas).").
				Gte(0).Lte(2).Default(0),

			// OrioleDB keeps its own in-memory page pool sized by
			// shared_buffers, so this knob dominates its behavior.
			// doc: https://www.orioledb.com/docs
			// doc: https://www.postgresql.org/docs/current/runtime-config-resource.html#GUC-SHARED-BUFFERS
			schemapb.Int64("shared_buffers_mb").Title("Shared buffers").Group("Memory").
				Desc("shared_buffers for the container; OrioleDB uses it as its primary page pool, so it should dominate the machine's RAM.").
				Unit("MB").Gte(128).Lte(1048576).Default(1024),

			// doc: https://www.postgresql.org/docs/current/app-initdb.html
			schemapb.Str("initdb_locale").Title("initdb locale").Group("Initialization").
				Desc("Locale passed to initdb inside the container; C.UTF-8 keeps collation cheap and image-independent.").
				Pattern(`^[A-Za-z0-9_@.\-]+$`).MaxLen(64).Default("C.UTF-8"),

			// doc: https://www.orioledb.com/docs/usage/getting-started
			schemapb.Bool("default_table_access_method").Title("OrioleDB by default").Group("Initialization").
				Desc("Set default_table_access_method = orioledb so unqualified CREATE TABLE lands on the OrioleDB engine rather than heap.").
				Default(true),

			// doc: https://www.postgresql.org/docs/current/app-psql.html
			schemapb.Str("init_sql").Title("Init SQL").Group("Initialization").
				Desc("SQL executed on the master once the container is healthy, before the workload.").
				MaxLen(65536),
		).
		Rules(
			schemapb.Rule(`int(root.haproxy) == 0 || int(root.replicas) >= 1`,
				"HAProxy in front of a single node only adds a hop").ID("haproxy-pointless").Warn(),
			schemapb.Rule(`int(root.shared_buffers_mb) >= 512 || int(root.replicas) == 0`,
				"a replica set with a tiny page pool measures the disk, not the engine").ID("buffers-vs-replicas").Warn(),
		).
		MustBuild()
}
