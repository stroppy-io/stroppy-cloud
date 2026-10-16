package dbkind

import (
	"math"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// PostgresLogin is the superuser the deployed PostgreSQL and OrioleDB
// stands are created with (the stand is throwaway; nothing here is a secret).
//
//nolint:gosec // G101: fixed benchmark-stand password, not a credential
var PostgresLogin = topology.Login{User: "postgres", Password: "stroppy_postgres", Database: "postgres"}

func postgres() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.Postgres, Title: "PostgreSQL", Description: "Single node, streaming replicas, Patroni HA, HAProxy and PgBouncer in front.",
			Versions: []catalog.Version{
				{Version: "18", Image: "postgres:18"},
				{Version: "17", Image: "postgres:17", Default: true},
				{Version: "16", Image: "postgres:16"},
				{Version: "15", Image: "postgres:15"},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Primary", Engine: "postgres", ConfigSchemas: []string{"cfg.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.patroni.yml@4", "cfg.exporter.postgres@1"}, ConfigSeeds: map[string]map[string]any{"cfg.postgresql.conf@17": {"extensions": []any{"pg_stat_statements"}}}},
				{Role: "db-replica", Title: "Replica", Engine: "postgres", ConfigSchemas: []string{"cfg.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.patroni.yml@4"}, ConfigSeeds: map[string]map[string]any{"cfg.postgresql.conf@17": {"extensions": []any{"pg_stat_statements"}}}},
				{Role: "etcd", Title: "etcd (Patroni DCS)", Engine: "etcd", ConfigSchemas: []string{"cfg.etcd@3"}},
				{Role: "proxy", Title: "HAProxy / PgBouncer", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2", "cfg.pgbouncer.ini@1"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Description: "One primary, no replicas.", Params: map[string]any{"version": "17"}},
				{ID: "primary-replica", Title: "Primary + replica", Description: "Streaming replication, one async replica.", Params: map[string]any{"version": "17", "replicas": 1}},
				{ID: "patroni-ha", Title: "Patroni HA", Description: "Three-node Patroni cluster with etcd and HAProxy.", Params: map[string]any{"version": "17", "replicas": 2, "sync_replicas": 1, "ha": "patroni", "etcd_nodes": 3, "haproxy": 1}},
				{ID: "pgbouncer", Title: "Single + PgBouncer", Description: "One primary behind PgBouncer.", Params: map[string]any{"version": "17", "pgbouncer": true, "haproxy": 1}},
			},
			ParamsSchema: "db.postgres.params@1", Protocols: []catalog.Protocol{catalog.ProtoPg}, Deployable: true,
		},
		plan:  func(p *topology.Plan, params map[string]any) { planPostgres(p, catalog.Postgres, params) },
		login: func(map[string]any) topology.Login { return PostgresLogin },
	}
}

func orioledb() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.OrioleDB, Title: "OrioleDB", Description: "PostgreSQL with the OrioleDB storage engine, deployed from the upstream image.",
			Versions: []catalog.Version{
				{Version: "beta17-pg18", Image: "orioledb/orioledb:beta17-pg18"},
				{Version: "beta17-pg17", Image: "orioledb/orioledb:beta17-pg17", Default: true},
				{Version: "beta17-pg16", Image: "orioledb/orioledb:beta17-pg16"},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Primary", Engine: "orioledb", ConfigSchemas: []string{"cfg.orioledb.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.docker.container@1"}, ConfigSeeds: map[string]map[string]any{"cfg.orioledb.postgresql.conf@17": {"extensions": []any{"pg_stat_statements", "orioledb"}}}},
				{Role: "db-replica", Title: "Replica", Engine: "orioledb", ConfigSchemas: []string{"cfg.orioledb.postgresql.conf@17", "cfg.pg_hba.conf@1", "cfg.docker.container@1"}, ConfigSeeds: map[string]map[string]any{"cfg.orioledb.postgresql.conf@17": {"extensions": []any{"pg_stat_statements", "orioledb"}}}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"image_tag": "beta17-pg17"}},
				{ID: "primary-replica", Title: "Primary + replica", Params: map[string]any{"image_tag": "beta17-pg17", "replicas": 1, "haproxy": 1}},
			},
			ParamsSchema: "db.orioledb.params@1", Protocols: []catalog.Protocol{catalog.ProtoPg}, Deployable: true,
		},
		plan:  func(p *topology.Plan, params map[string]any) { planPostgres(p, catalog.OrioleDB, params) },
		login: func(map[string]any) topology.Login { return PostgresLogin },
	}
}

func planPostgres(p *topology.Plan, kind catalog.DatabaseKind, params map[string]any) {
	engine := "postgres"
	if kind == catalog.OrioleDB {
		engine = "orioledb"
	}
	replicas := topology.IntOf(params, "replicas", 0)
	ha := topology.StrOf(params, "ha", "none")
	haproxy := topology.IntOf(params, "haproxy", 0)
	pgbouncer := topology.BoolOf(params, "pgbouncer")

	p.Add(topology.RoleDB, engine, 1)
	p.Add(topology.RoleDBReplica, engine, replicas)
	if ha == "patroni" {
		p.Add(topology.RoleEtcd, "etcd", topology.IntOf(params, "etcd_nodes", 3))
		p.Colocate(topology.RolePatroni, "patroni", topology.RoleDB)
		p.Colocate(topology.RolePatroniReplica, "patroni", topology.RoleDBReplica)
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleEtcd, To: topology.RoleEtcd, Protocol: "etcd-peer", Port: 2380})
		for _, from := range []string{topology.RoleDB, topology.RoleDBReplica} {
			for _, to := range []string{topology.RoleDB, topology.RoleDBReplica} {
				p.Flows = append(p.Flows, topology.Flow{From: from, To: to, Protocol: "patroni-api", Port: 8008}, topology.Flow{From: from, To: to, Protocol: "pg-streaming", Port: 5432})
			}
			if haproxy > 0 {
				p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: from, Protocol: "patroni-api", Port: 8008})
			}
		}
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDB, To: topology.RoleEtcd, Protocol: "etcd", Port: 2379}, topology.Flow{From: topology.RoleDBReplica, To: topology.RoleEtcd, Protocol: "etcd", Port: 2379})
	}
	if replicas > 0 {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDBReplica, To: topology.RoleDB, Protocol: "pg-streaming", Port: 5432})
	}
	client := topology.Endpoint{Role: topology.RoleDB, Protocol: "pg", Port: 5432}
	switch {
	case haproxy > 0:
		p.Add(topology.RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDB, Protocol: "pg", Port: 5432})
		if replicas > 0 {
			p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDBReplica, Protocol: "pg", Port: 5432})
		}
		client = topology.Endpoint{Role: topology.RoleProxy, Protocol: "pg", Port: 5000}
		if pgbouncer {
			p.Colocate(topology.RolePgBouncer, "pgbouncer", topology.RoleProxy)
			client.Port = 6432
		}
	case pgbouncer:
		p.Colocate(topology.RolePgBouncer, "pgbouncer", topology.RoleDB)
		client.Port = 6432
	}
	p.Client = client

	dbReq := topology.Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "postgres baseline"}
	if topology.BoolOf(params, "wal_archive") {
		dbReq.DiskGB += 20
		dbReq.Reason = "postgres baseline + WAL archive"
	}
	if kind == catalog.OrioleDB {
		sb := float64(topology.IntOf(params, "shared_buffers_mb", 1024)) / 1024
		if sb*2 > dbReq.MemoryGB {
			dbReq.MemoryGB = math.Ceil(sb * 2)
			dbReq.Reason = "2× shared_buffers"
		}
	}
	p.Requirements[topology.RoleDB] = dbReq
	if replicas > 0 {
		p.Requirements[topology.RoleDBReplica] = dbReq
	}
	if ha == "patroni" {
		p.Requirements[topology.RoleEtcd] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "etcd"}
	}
	if haproxy > 0 {
		p.Requirements[topology.RoleProxy] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "haproxy"}
	}
	p.Label = topology.Label(engine, replicas, "replica", ha == "patroni", "patroni", haproxy > 0, "haproxy", pgbouncer, "pgbouncer")
}
