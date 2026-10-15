package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// MariaDB is db.mariadb.params@1 — a self-hosted MariaDB stand. It shares the
// MySQL wire protocol but not its clustering: MariaDB's multi-master story is
// Galera, and its proxy of choice is MaxScale next to ProxySQL.
//
// doc: https://mariadb.com/docs/server
func MariaDB() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("mariadb", 1)).
		Descr("MariaDB topology and options: version, replicas, replication mode (incl. Galera), proxies.").
		Strict().Coerce().
		Fields(
			// Community LTS lines alive on 2026-09-08: 11.8 (EOL 2028-06),
			// 11.4 (EOL 2029-05), 10.11 (EOL 2028-02). 10.6 went EOL
			// 2026-07-06 and is gone.
			// doc: https://mariadb.org/about/#maintenance-policy
			schemapb.Choice("version").Title("MariaDB version").Group("Engine").
				Desc("Server LTS series; packages come from the mariadb_repo_setup repository.").
				Opt(schemapb.StrV("11.8"), "11.8 LTS").
				Opt(schemapb.StrV("11.4"), "11.4 LTS").
				Opt(schemapb.StrV("10.11"), "10.11 LTS").
				Default(schemapb.StrV("11.4")).Required(),

			// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication
			schemapb.Int64("replicas").Title("Replicas").Group("Topology").
				Desc("Secondary servers behind the primary; not used in Galera mode, where the cluster is sized by galera_nodes instead.").
				Gte(0).Lte(8).Default(0),

			// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication
			// doc: https://mariadb.com/docs/galera-cluster
			schemapb.Choice("replication").Title("Replication mode").Group("Replication").
				Desc("async/semi_sync = one primary with standard MariaDB replication; galera = a synchronous multi-master Galera cluster with no distinguished primary.").
				Opt(schemapb.StrV("async"), "Asynchronous").
				Opt(schemapb.StrV("semi_sync"), "Semi-synchronous").
				Opt(schemapb.StrV("galera"), "Galera (multi-master)").
				Default(schemapb.StrV("async")).Required(),

			// Quorum is a strict majority of the last known membership, so an
			// even cluster buys nothing and risks split brain.
			// doc: https://mariadb.com/docs/galera-cluster/high-availability/understanding-quorum-monitoring-and-recovery
			schemapb.Choice("galera_nodes").Title("Galera nodes").Group("Replication").
				Desc("Size of the Galera cluster; odd only, minimum 3 — quorum needs a strict majority of the last known membership.").
				Opt(schemapb.Int64V(3), "3 (tolerates one loss)").
				Opt(schemapb.Int64V(5), "5 (tolerates two losses)").
				Opt(schemapb.Int64V(7), "7 (tolerates three losses)").
				Default(schemapb.Int64V(3)).
				When(`root.replication == "galera"`),

			// doc: https://proxysql.com/documentation/
			schemapb.Int64("proxysql").Title("ProxySQL nodes").Group("Routing").
				Desc("ProxySQL instances in front of the cluster; mutually exclusive with MaxScale.").
				Gte(0).Lte(2).Default(0),

			// doc: https://mariadb.com/docs/maxscale
			// License: MaxScale is BSL — free in production only below three
			// database servers (see cfg/maxscale.go).
			schemapb.Bool("maxscale").Title("MaxScale").Group("Routing").
				Desc("Run MariaDB MaxScale 24.02 as the router instead of ProxySQL: read/write split and Galera monitoring out of the box. License: MaxScale is under the Business Source License — free in production only with fewer than three database servers; a replicated or Galera cluster behind it needs a MariaDB subscription outside benchmarking.").
				Default(false),

			// doc: https://mariadb.com/docs/server/clients-and-utilities/basic-sql-client/mariadb-command-line-client
			schemapb.Str("init_sql").Title("Init SQL").Group("Initialization").
				Desc("SQL executed on the primary (or the first Galera node) once the server is up, before the workload.").
				MaxLen(65536),
		).
		Rules(
			schemapb.Rule(`root.replication != "galera" || int(root.replicas) == 0`,
				"Galera has no replicas: size the cluster with galera_nodes").ID("galera-no-replicas"),
			schemapb.Rule(`root.replication != "semi_sync" || int(root.replicas) >= 1`,
				"semi-synchronous replication needs at least one replica").ID("semisync-needs-replica"),
			schemapb.Rule(`!(root.maxscale && int(root.proxysql) > 0)`,
				"pick one router: MaxScale or ProxySQL, not both").ID("one-router"),
			schemapb.Rule(`int(root.proxysql) == 0 || root.replication == "galera" || int(root.replicas) >= 1`,
				"ProxySQL in front of a single server only adds a hop").ID("proxysql-pointless").Warn(),
		).
		MustBuild()
}
