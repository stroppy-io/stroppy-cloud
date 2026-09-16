package catalog

// databases is the v0 database matrix. Versions mirror the choices of the
// db.<kind>.params@1 schemas (pipelines/schemas/dbparams); roles are the
// machine roles the topology compiler emits and the config schemas each
// role's containers render from.
//
// Images: official upstream images at the major/minor tag; the run
// resolves the exact digest at launch.
func databases() []Database {
	return []Database{
		{
			Kind: Postgres, Title: "PostgreSQL", Description: "Single node, streaming replicas, Patroni HA, HAProxy and PgBouncer in front.",
			Versions: []Version{
				{Version: "18", Image: "postgres:18"},
				{Version: "17", Image: "postgres:17", Default: true},
				{Version: "16", Image: "postgres:16"},
				{Version: "15", Image: "postgres:15"},
			},
			Roles: []Role{
				{Role: "db", Title: "Primary", Engine: "postgres", ConfigSchemas: []string{"cfg.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.patroni.yml@4", "cfg.exporter.postgres@1"}, ConfigSeeds: map[string]map[string]any{"cfg.postgresql.conf@17": {"extensions": []any{"pg_stat_statements"}}}},
				{Role: "db-replica", Title: "Replica", Engine: "postgres", ConfigSchemas: []string{"cfg.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.patroni.yml@4"}, ConfigSeeds: map[string]map[string]any{"cfg.postgresql.conf@17": {"extensions": []any{"pg_stat_statements"}}}},
				{Role: "etcd", Title: "etcd (Patroni DCS)", Engine: "etcd", ConfigSchemas: []string{"cfg.etcd@3"}},
				{Role: "proxy", Title: "HAProxy / PgBouncer", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2", "cfg.pgbouncer.ini@1"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Description: "One primary, no replicas.", Params: map[string]any{"version": "17"}},
				{ID: "primary-replica", Title: "Primary + replica", Description: "Streaming replication, one async replica.", Params: map[string]any{"version": "17", "replicas": 1}},
				{ID: "patroni-ha", Title: "Patroni HA", Description: "Three-node Patroni cluster with etcd and HAProxy.", Params: map[string]any{"version": "17", "replicas": 2, "sync_replicas": 1, "ha": "patroni", "etcd_nodes": 3, "haproxy": 1}},
				{ID: "pgbouncer", Title: "Single + PgBouncer", Description: "One primary behind PgBouncer.", Params: map[string]any{"version": "17", "pgbouncer": true, "haproxy": 1}},
			},
			ParamsSchema: "db.postgres.params@1", Protocols: []Protocol{ProtoPg}, Deployable: true,
		},
		{
			Kind: OrioleDB, Title: "OrioleDB", Description: "PostgreSQL with the OrioleDB storage engine, deployed from the upstream image.",
			Versions: []Version{
				{Version: "beta17-pg18", Image: "orioledb/orioledb:beta17-pg18"},
				{Version: "beta17-pg17", Image: "orioledb/orioledb:beta17-pg17", Default: true},
				{Version: "beta17-pg16", Image: "orioledb/orioledb:beta17-pg16"},
			},
			Roles: []Role{
				{Role: "db", Title: "Primary", Engine: "orioledb", ConfigSchemas: []string{"cfg.orioledb.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.docker.container@1"}, ConfigSeeds: map[string]map[string]any{"cfg.orioledb.postgresql.conf@17": {"extensions": []any{"pg_stat_statements", "orioledb"}}}},
				{Role: "db-replica", Title: "Replica", Engine: "orioledb", ConfigSchemas: []string{"cfg.orioledb.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.docker.container@1"}, ConfigSeeds: map[string]map[string]any{"cfg.orioledb.postgresql.conf@17": {"extensions": []any{"pg_stat_statements", "orioledb"}}}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"image_tag": "beta17-pg17"}},
				{ID: "primary-replica", Title: "Primary + replica", Params: map[string]any{"image_tag": "beta17-pg17", "replicas": 1, "haproxy": 1}},
			},
			ParamsSchema: "db.orioledb.params@1", Protocols: []Protocol{ProtoPg}, Deployable: true,
		},
		{
			Kind: MySQL, Title: "MySQL", Description: "Single node, async/semi-sync replicas or Group Replication, ProxySQL in front.",
			Versions: []Version{
				{Version: "8.4", Image: "mysql:8.4", Default: true},
				{Version: "8.0", Image: "mysql:8.0", Deprecated: true},
			},
			Roles: []Role{
				{Role: "db", Title: "Primary", Engine: "mysql", ConfigSchemas: []string{"cfg.my.cnf@8.4", "cfg.exporter.mysqld@1"}},
				{Role: "db-replica", Title: "Replica", Engine: "mysql", ConfigSchemas: []string{"cfg.my.cnf@8.4"}},
				{Role: "proxy", Title: "ProxySQL", Engine: "proxysql", ConfigSchemas: []string{"cfg.proxysql.cnf@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "8.4"}},
				{ID: "semi-sync", Title: "Primary + semi-sync replica", Params: map[string]any{"version": "8.4", "replicas": 1, "replication": "semi_sync"}},
				{ID: "group-replication", Title: "Group Replication", Description: "Three members, single-primary, ProxySQL routing.", Params: map[string]any{"version": "8.4", "replicas": 2, "replication": "group", "proxysql": 1}},
			},
			ParamsSchema: "db.mysql.params@1", Protocols: []Protocol{ProtoMySQL}, Deployable: true,
		},
		{
			Kind: MariaDB, Title: "MariaDB", Description: "Single node, async/semi-sync replicas or Galera, ProxySQL or MaxScale in front.",
			Versions: []Version{
				{Version: "11.8", Image: "mariadb:11.8"},
				{Version: "11.4", Image: "mariadb:11.4", Default: true},
				{Version: "10.11", Image: "mariadb:10.11"},
			},
			Roles: []Role{
				{Role: "db", Title: "Primary", Engine: "mariadb", ConfigSchemas: []string{"cfg.mariadb.cnf@11.4", "cfg.exporter.mysqld@1"}},
				{Role: "db-replica", Title: "Replica / Galera node", Engine: "mariadb", ConfigSchemas: []string{"cfg.mariadb.cnf@11.4"}},
				{Role: "proxy", Title: "ProxySQL / MaxScale", Engine: "proxysql", ConfigSchemas: []string{"cfg.proxysql.cnf@2", "cfg.maxscale.cnf@25"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "11.4"}},
				{ID: "galera", Title: "Galera cluster", Description: "Three multi-master nodes behind ProxySQL.", Params: map[string]any{"version": "11.4", "replication": "galera", "galera_nodes": 3, "proxysql": 1}},
			},
			ParamsSchema: "db.mariadb.params@1", Protocols: []Protocol{ProtoMySQL}, Deployable: true,
		},
		{
			Kind: Picodata, Title: "Picodata", Description: "Distributed in-memory SQL; tiers of instances with replication factors, pgproto for stroppy.",
			Versions: []Version{
				{Version: "26.2", Image: "docker.binary.picodata.io/picodata:26.2"},
				{Version: "26.1", Image: "docker.binary.picodata.io/picodata:26.1", Default: true},
				{Version: "25.3", Image: "docker.binary.picodata.io/picodata:25.3.8", Deprecated: true},
			},
			Roles: []Role{
				{Role: "db", Title: "Picodata instance", Engine: "picodata", ConfigSchemas: []string{"cfg.picodata.yaml@26"}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single instance", Params: map[string]any{"version": "26.1", "tiers": []any{map[string]any{"name": "default", "instances": 1}}}},
				{ID: "three-node", Title: "Three instances, RF 3", Params: map[string]any{"version": "26.1", "tiers": []any{map[string]any{"name": "default", "instances": 3, "replication_factor": 3}}, "haproxy": 1}},
			},
			ParamsSchema: "db.picodata.params@1", Protocols: []Protocol{ProtoPicodata, ProtoPg}, Deployable: true,
		},
		{
			Kind: YDB, Title: "YDB", Description: "Self-hosted YDB: storage and database nodes, erasure modes, pdisks per node.",
			Versions: []Version{
				{Version: "26.3", Image: "ydbplatform/local-ydb:26.3.1.14"},
				{Version: "26.2", Image: "ydbplatform/local-ydb:26.2.1.14", Default: true},
				{Version: "26.1", Image: "ydbplatform/local-ydb:26.1.1.22"},
				{Version: "25.4", Image: "ydbplatform/local-ydb:25.4.1.15"},
			},
			Roles: []Role{
				{Role: "db", Title: "Storage node", Engine: "ydb", ConfigSchemas: []string{"cfg.ydb.config.yaml@26", "cfg.host.disks@1"}},
				{Role: "db-compute", Title: "Database node", Engine: "ydb", ConfigSchemas: []string{"cfg.ydb.config.yaml@26"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "26.2"}},
				{ID: "mirror-3-dc", Title: "mirror-3-dc", Description: "Nine storage nodes, three pdisks each, three database nodes.", Params: map[string]any{"version": "26.2", "fault_tolerance": "mirror-3-dc", "failure_domain": "disk", "storage_nodes": 9, "database_nodes": 3, "pdisks_per_node": 3}},
			},
			ParamsSchema: "db.ydb.params@1", Protocols: []Protocol{ProtoYDBGrpc, ProtoYDBGrpcs}, Deployable: true,
		},
		{
			Kind: YDBManaged, Title: "YDB (Yandex Managed)", Description: "Managed Service for YDB: dedicated or serverless database created by the run.",
			Versions: []Version{{Version: "managed", Default: true}},
			Roles: []Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "dedicated-medium", Title: "Dedicated, medium ×3", Params: map[string]any{"type": "dedicated", "resource_preset_id": "medium", "node_count": 3}},
				{ID: "serverless", Title: "Serverless", Params: map[string]any{"type": "serverless"}},
			},
			ParamsSchema: "db.ydb_managed.params@1", Protocols: []Protocol{ProtoYDBGrpcs}, Deployable: true,
		},
		{
			Kind: Cockroach, Title: "CockroachDB", Description: "Multi-node cluster, insecure mode for benchmarks, HAProxy in front.",
			Versions: []Version{
				{Version: "26.3", Image: "cockroachdb/cockroach:v26.3.1"},
				{Version: "26.2", Image: "cockroachdb/cockroach:v26.2.6"},
				{Version: "25.4", Image: "cockroachdb/cockroach:v25.4.16", Default: true},
				{Version: "25.2", Image: "cockroachdb/cockroach:v25.2.23"},
				{Version: "24.3", Image: "cockroachdb/cockroach:v24.3.36"},
				{Version: "24.1", Image: "cockroachdb/cockroach:v24.1.33", Deprecated: true},
			},
			Roles: []Role{
				{Role: "db", Title: "Cockroach node", Engine: "cockroach", ConfigSchemas: []string{"cfg.cockroach.flags@25"}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "25.4", "nodes": 1}},
				{ID: "three-node", Title: "Three nodes + HAProxy", Params: map[string]any{"version": "25.4", "nodes": 3, "haproxy": 1}},
			},
			ParamsSchema: "db.cockroach.params@1", Protocols: []Protocol{ProtoCockroach, ProtoPg}, Deployable: true,
		},
		{
			Kind: PgNoop, Title: "pg-noop", Description: "PostgreSQL-protocol server that acknowledges everything: the driver-side ceiling.",
			Versions: []Version{{Version: "0.1.2", Image: "docker.stroppy.io/stroppy-io/pg-noop@sha256:c35aea48379fcefadb8f06cab4e6023257f28c23f3f27855680f97e468df4f3d", Default: true}},
			Roles: []Role{
				{Role: "db", Title: "pg-noop", Engine: "pg_noop"},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []Topology{{ID: "single", Title: "Single node", Params: map[string]any{"version": "0.1.2"}}},
			ParamsSchema: "db.pg_noop.params@1", Protocols: []Protocol{ProtoPg}, Deployable: true,
		},
		{
			Kind: Noop, Title: "Noop", Description: "No database at all: stroppy's own generator, the framework ceiling.",
			Versions: []Version{{Version: "builtin", Default: true}},
			Roles: []Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []Topology{{ID: "runner-only", Title: "Runner only", Params: map[string]any{}}},
			ParamsSchema: "db.noop.params@1", Protocols: []Protocol{ProtoNoop}, Deployable: true,
		},
		{
			Kind: External, Title: "External database", Description: "A database you already run: the platform only places the runner and connects.",
			Versions: []Version{{Version: "external", Default: true}},
			Roles: []Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []Topology{{ID: "dsn", Title: "By DSN", Params: map[string]any{"protocol": "pg", "dsn": "postgres://stroppy@db.example.internal:5432/stroppy"}}},
			ParamsSchema: "db.external.params@1",
			Protocols:    []Protocol{ProtoPg, ProtoMySQL, ProtoPicodata, ProtoYDBGrpc, ProtoYDBGrpcs, ProtoCockroach},
			Deployable:   false,
		},
	}
}
