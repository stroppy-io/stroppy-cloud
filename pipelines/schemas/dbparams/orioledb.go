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
			// Tag list verified against the registry on 2026-10-22: the
			// repository publishes latest-pgNN (tracking main), betaNN-pgNN
			// releases (beta17, beta18, beta19) and pgNN-nightly for majors
			// 16, 17, 18. beta18 bumped the WAL format: primary and replicas
			// of one stand always run the same tag.
			// The project calls itself public beta — not production software,
			// which is fine for a benchmark stand.
			// doc: https://hub.docker.com/r/orioledb/orioledb/tags
			schemapb.Choice("image_tag").Title("Image tag").Group("Engine").
				Desc("Tag of the orioledb/orioledb image; the pgNN suffix picks the PostgreSQL major the engine is patched into.").
				Opt(schemapb.StrV("latest-pg18"), "latest-pg18 (main, PG 18)").
				Opt(schemapb.StrV("latest-pg17"), "latest-pg17 (main, PG 17)").
				Opt(schemapb.StrV("latest-pg16"), "latest-pg16 (main, PG 16)").
				Opt(schemapb.StrV("beta19-pg18"), "beta19-pg18 (release, PG 18)").
				Opt(schemapb.StrV("beta19-pg17"), "beta19-pg17 (release, PG 17)").
				Opt(schemapb.StrV("beta19-pg16"), "beta19-pg16 (release, PG 16)").
				Opt(schemapb.StrV("beta18-pg18"), "beta18-pg18 (release, PG 18)").
				Opt(schemapb.StrV("beta18-pg17"), "beta18-pg17 (release, PG 17)").
				Opt(schemapb.StrV("beta18-pg16"), "beta18-pg16 (release, PG 16)").
				Opt(schemapb.StrV("beta17-pg18"), "beta17-pg18 (release, PG 18)").
				Opt(schemapb.StrV("beta17-pg17"), "beta17-pg17 (release, PG 17)").
				Opt(schemapb.StrV("beta17-pg16"), "beta17-pg16 (release, PG 16)").
				Default(schemapb.StrV("beta19-pg17")).Required(),

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

			// OrioleDB tables are cached in orioledb.main_buffers, not in
			// shared_buffers (which only holds heap pages: the catalog).
			// doc: https://www.orioledb.com/docs/usage/configuration — orioledb.main_buffers
			schemapb.Int64("main_buffers_mb").Title("Main buffers").Group("Memory").
				Desc("orioledb.main_buffers of every database server, the page cache of OrioleDB tables. Unset = auto tuning sizes it from the machine (30% of RAM); stock tuning keeps the 64MB upstream default. A main_buffers set in the config wins.").
				Unit("MB").Gte(64).Lte(4194304).Nullable(),

			// Retired: it never reached the config, and shared_buffers is not
			// OrioleDB's page pool. Declared (never active) so stored
			// definitions that carry it still load under Strict.
			schemapb.Int64("shared_buffers_mb").Title("Shared buffers").Group("Memory").
				Desc("Retired: replaced by main_buffers_mb; a stored value is ignored.").
				Unit("MB").Deprecated().When("false"),

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
			schemapb.Rule(`!("main_buffers_mb" in root) || int(root.main_buffers_mb) >= 512 || int(root.replicas) == 0`,
				"a replica set with a tiny page pool measures the disk, not the engine").ID("buffers-vs-replicas").Warn(),
		).
		MustBuild()
}
