package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// MySQL is db.mysql.params@1 — a self-hosted MySQL stand: version, how many
// replicas, which replication flavor ties them together, and how many
// ProxySQL hops sit in front. my.cnf itself lives in cfg.mysql.
//
// doc: https://dev.mysql.com/doc/refman/8.4/en/
func MySQL() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("mysql", 1)).
		Descr("MySQL topology and options: version, replicas, replication mode, ProxySQL.").
		Strict().Coerce().
		Fields(
			// 8.4 is the current LTS (GA 2024-04, premier support to 2029-04);
			// 8.0 left premier support in 2025-04 and is offered only for
			// comparison against historical numbers.
			// doc: https://dev.mysql.com/doc/refman/8.4/en/
			// doc: https://endoflife.date/mysql
			schemapb.Choice("version").Title("MySQL version").Group("Engine").
				Desc("Server series; packages come from repo.mysql.com.").
				Opt(schemapb.StrV("8.4"), "8.4 LTS").
				Opt(schemapb.StrV("8.0"), "8.0 (extended support only)").
				Default(schemapb.StrV("8.4")).Required(),

			// doc: https://dev.mysql.com/doc/refman/8.4/en/replication.html
			schemapb.Int64("replicas").Title("Replicas").Group("Topology").
				Desc("Secondary servers besides the primary; with Group Replication the whole group is primary + replicas and may not exceed 9 members.").
				Gte(0).Lte(8).Default(0),

			// Group Replication is not built on the semisync plugin: it has its
			// own group communication layer, hence three exclusive flavors
			// rather than orthogonal toggles.
			// doc: https://dev.mysql.com/doc/refman/8.4/en/replication-semisync.html
			// doc: https://dev.mysql.com/doc/refman/8.4/en/group-replication.html
			schemapb.Choice("replication").Title("Replication mode").Group("Replication").
				Desc("async = classic binlog replication; semi_sync = the semisync plugin waits for one replica ack; group = Group Replication (its own consensus, no semisync).").
				Opt(schemapb.StrV("async"), "Asynchronous").
				Opt(schemapb.StrV("semi_sync"), "Semi-synchronous").
				Opt(schemapb.StrV("group"), "Group Replication").
				Default(schemapb.StrV("async")).Required(),

			// doc: https://dev.mysql.com/doc/refman/8.4/en/group-replication-single-primary-mode.html
			schemapb.Bool("single_primary").Title("Single-primary group").Group("Replication").
				Desc("Group Replication mode: on = one writable primary with automatic election, off = multi-primary (every member writable).").
				Default(true).When(`root.replication == "group"`),

			// The group is capped at 9 members; a tenth join request is refused
			// by the group itself.
			// doc: https://dev.mysql.com/doc/refman/8.4/en/group-replication-limitations.html
			schemapb.Int64("semi_sync_wait_for_slave_count").Title("Semi-sync acks").Group("Replication").
				Desc("rpl_semi_sync_source_wait_for_replica_count: how many replicas must acknowledge before the primary commits.").
				Gte(1).Lte(8).Default(1).When(`root.replication == "semi_sync"`),

			// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-buffer-pool-resize.html
			// doc: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-buffer-pool
			// The upstream default (128 MB) reads every benchmark table from
			// disk; MySQL recommends up to ~80% of RAM on a dedicated server.
			schemapb.Choice("buffer_pool").Title("Buffer pool").Group("Memory").
				Desc("auto = 70% of the database machine's RAM (and a disk-class innodb_io_capacity); fixed = buffer_pool_mb; stock = the server's own defaults (128 MB). An innodb_buffer_pool_size set in the config wins over every mode.").
				Opt(schemapb.StrV("auto"), "70% of RAM").
				Opt(schemapb.StrV("fixed"), "Fixed size").
				Opt(schemapb.StrV("stock"), "Server default").
				Default(schemapb.StrV("auto")),
			schemapb.Int64("buffer_pool_mb").Title("Buffer pool size").Group("Memory").
				Desc("innodb_buffer_pool_size of every database server; the run refuses a size over the machine's RAM.").
				Unit("MB").Gte(128).Lte(4194304).Default(4096).When(`root.buffer_pool == "fixed"`),

			// doc: https://proxysql.com/documentation/
			schemapb.Int64("proxysql").Title("ProxySQL nodes").Group("Routing").
				Desc("ProxySQL instances routing reads and writes to the right member; 0 = clients talk to the primary directly.").
				Gte(0).Lte(2).Default(0),

			// doc: https://dev.mysql.com/doc/refman/8.4/en/mysql.html
			schemapb.Str("init_sql").Title("Init SQL").Group("Initialization").
				Desc("SQL executed on the primary once the server is up, before the workload.").
				MaxLen(65536),
		).
		Rules(
			schemapb.Rule(`root.replication != "group" || (int(root.replicas) + 1 >= 3 && int(root.replicas) + 1 <= 9)`,
				"Group Replication needs 3..9 members, i.e. replicas between 2 and 8").ID("group-members"),
			schemapb.Rule(`root.replication == "async" || int(root.replicas) >= 1`,
				"semi-synchronous and Group Replication both need at least one replica").ID("replication-needs-replica"),
			schemapb.Rule(`!("semi_sync_wait_for_slave_count" in root) || root.replication != "semi_sync" || int(root.semi_sync_wait_for_slave_count) <= int(root.replicas)`,
				"cannot wait for more acks than there are replicas").ID("semi-sync-acks-le-replicas"),
			schemapb.Rule(`int(root.proxysql) == 0 || int(root.replicas) >= 1`,
				"ProxySQL in front of a single server only adds a hop").ID("proxysql-pointless").Warn(),
		).
		MustBuild()
}
