package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Postgres is db.postgres.params@1 — the SHAPE of a self-hosted PostgreSQL
// stand: engine version, how many nodes of which role, which HA machinery is
// wired in. Hardware (cpu/ram/disk) comes from Test sizes, and the contents of
// postgresql.conf / pg_hba.conf / patroni.yml from the cfg.* schemas.
//
// doc: https://www.postgresql.org/support/versioning/
func Postgres() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("postgres", 1)).
		Descr("PostgreSQL topology and options: version, replicas, HA machinery, extensions.").
		Strict().Coerce().
		Fields(
			tuning(),
			// Community-supported majors as of 2026-09: 14 (EOL 2026-11-12),
			// 15, 16, 17, 18. 14 is dropped — it dies inside a benchmark
			// season; 19 is still beta and not offered.
			// doc: https://www.postgresql.org/support/versioning/
			schemapb.Choice("version").Title("PostgreSQL version").Group("Engine").
				Desc("Server major version; packages come from the PGDG apt repository (apt.postgresql.org).").
				Opt(schemapb.StrV("18"), "18").
				Opt(schemapb.StrV("17"), "17").
				Opt(schemapb.StrV("16"), "16").
				Opt(schemapb.StrV("15"), "15").
				Default(schemapb.StrV("17")).Required(),

			// doc: https://www.postgresql.org/docs/current/warm-standby.html
			schemapb.Int64("replicas").Title("Streaming replicas").Group("Topology").
				Desc("Physical streaming standbys next to the primary; 0 = a single node.").
				Gte(0).Lte(8).Default(0),

			// doc: https://www.postgresql.org/docs/current/runtime-config-replication.html#GUC-SYNCHRONOUS-STANDBY-NAMES
			schemapb.Int64("sync_replicas").Title("Synchronous replicas").Group("Topology").
				Desc("How many of the replicas commit synchronously (synchronous_standby_names); must not exceed replicas.").
				Gte(0).Lte(8).Default(0),

			// doc: https://patroni.readthedocs.io/en/latest/README.html
			schemapb.Choice("ha").Title("High availability").Group("High availability").
				Desc("none = plain primary/standby without failover; patroni = Patroni owns postgres and elects the leader through etcd.").
				Opt(schemapb.StrV("none"), "None (manual)").
				Opt(schemapb.StrV("patroni"), "Patroni + etcd").
				Default(schemapb.StrV("none")).Required(),

			// doc: https://etcd.io/docs/v3.5/faq/#why-an-odd-number-of-cluster-members
			schemapb.Choice("etcd_nodes").Title("etcd nodes").Group("High availability").
				Desc("Size of the etcd DCS backing Patroni; odd numbers only — 1 for a toy stand, 3 to survive one loss.").
				Opt(schemapb.Int64V(1), "1 (no fault tolerance)").
				Opt(schemapb.Int64V(3), "3 (tolerates one loss)").
				Default(schemapb.Int64V(3)).
				When(`root.ha == "patroni"`),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances routing clients to the current primary (httpchk /primary against the Patroni REST API).").
				Gte(0).Lte(2).Default(0),

			// doc: https://www.pgbouncer.org/config.html
			schemapb.Bool("pgbouncer").Title("PgBouncer").Group("Routing").
				Desc("Collocate a PgBouncer pooler with every database node.").
				Default(false),

			// doc: https://www.postgresql.org/docs/current/continuous-archiving.html
			schemapb.Bool("wal_archive").Title("WAL archiving").Group("Storage").
				Desc("Turn on archive_mode and archive WAL segments to local storage on the primary.").
				Default(false),

			// Only extensions that are either bundled contrib or a single PGDG
			// apt package postgresql-<major>-<name>; anything needing a foreign
			// repository (timescaledb) is out.
			// doc: https://www.postgresql.org/docs/current/contrib.html
			// doc: https://wiki.postgresql.org/wiki/Apt
			schemapb.List("extensions",
				schemapb.Choice("").
					// contrib, ships with the server
					// doc: https://www.postgresql.org/docs/current/pgstatstatements.html
					Opt(schemapb.StrV("pg_stat_statements"), "pg_stat_statements (contrib)").
					// doc: https://www.postgresql.org/docs/current/pgbuffercache.html
					Opt(schemapb.StrV("pg_buffercache"), "pg_buffercache (contrib)").
					// doc: https://www.postgresql.org/docs/current/pgprewarm.html
					Opt(schemapb.StrV("pg_prewarm"), "pg_prewarm (contrib)").
					// doc: https://www.postgresql.org/docs/current/pgtrgm.html
					Opt(schemapb.StrV("pg_trgm"), "pg_trgm (contrib)").
					// doc: https://github.com/pgvector/pgvector — apt postgresql-<major>-pgvector
					Opt(schemapb.StrV("vector"), "pgvector (postgresql-N-pgvector)").
					// doc: https://github.com/pgpartman/pg_partman — apt postgresql-<major>-partman
					Opt(schemapb.StrV("pg_partman"), "pg_partman (postgresql-N-partman)").
					// doc: https://postgis.net/install/ — apt postgresql-<major>-postgis-3
					Opt(schemapb.StrV("postgis"), "PostGIS 3 (postgresql-N-postgis-3)").
					// doc: https://github.com/HypoPG/hypopg — apt postgresql-<major>-hypopg
					Opt(schemapb.StrV("hypopg"), "hypopg (postgresql-N-hypopg)").
					// doc: https://github.com/citusdata/pg_cron — apt postgresql-<major>-cron
					Opt(schemapb.StrV("pg_cron"), "pg_cron (postgresql-N-cron)"),
			).Title("Extensions").Group("Extensions").
				Desc("CREATE EXTENSION on the benchmark database after initdb; preload-only ones are added to shared_preload_libraries too.").
				Unique().MaxItems(9),

			// doc: https://www.postgresql.org/docs/current/app-initdb.html
			schemapb.Str("locale").Title("initdb locale").Group("Initialization").
				Desc("Locale passed to initdb --locale; C.UTF-8 keeps collation cheap and stable across images.").
				Pattern(`^[A-Za-z0-9_@.\-]+$`).MaxLen(64).Default("C.UTF-8"),

			// doc: https://www.postgresql.org/docs/current/app-psql.html
			schemapb.Str("init_sql").Title("Init SQL").Group("Initialization").
				Desc("SQL executed on the primary once the cluster is up, before the workload (schema tweaks, roles, GUC overrides).").
				MaxLen(65536),
		).
		Rules(
			schemapb.Rule(`int(root.sync_replicas) <= int(root.replicas)`,
				"sync_replicas must not exceed replicas").ID("sync-le-replicas"),
			schemapb.Rule(`root.ha != "patroni" || int(root.replicas) >= 1`,
				"Patroni needs at least one replica to fail over to").ID("patroni-needs-replica"),
			schemapb.Rule(`int(root.haproxy) == 0 || int(root.replicas) >= 1 || root.pgbouncer`,
				"HAProxy in front of a single unpooled node adds a hop and measures nothing").
				ID("haproxy-pointless").Warn(),
			schemapb.Rule(`int(root.sync_replicas) == 0 || root.ha == "patroni" || int(root.replicas) >= 2`,
				"synchronous commit without Patroni and with a single replica stalls the primary when that replica dies").
				ID("sync-single-replica").Warn(),
		).
		MustBuild()
}
