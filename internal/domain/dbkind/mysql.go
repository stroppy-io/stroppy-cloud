package dbkind

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// MySQLLogin is the root account of the deployed MySQL and MariaDB stands.
//
//nolint:gosec // G101: fixed benchmark-stand password, not a credential
var MySQLLogin = topology.Login{User: "root", Password: "stroppy_mysql", Database: "stroppy"}

func mysql() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.MySQL, Title: "MySQL", Description: "Single node, async/semi-sync replicas or Group Replication, ProxySQL in front.",
			Versions: []catalog.Version{
				{Version: "8.4", Image: "mysql:8.4", Default: true},
				{Version: "8.0", Image: "mysql:8.0", Deprecated: true},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Primary", Engine: "mysql", ConfigSchemas: []string{"cfg.my.cnf@8.4", "cfg.exporter.mysqld@1"}},
				{Role: "db-replica", Title: "Replica", Engine: "mysql", ConfigSchemas: []string{"cfg.my.cnf@8.4"}},
				{Role: "proxy", Title: "ProxySQL", Engine: "proxysql", ConfigSchemas: []string{"cfg.proxysql.cnf@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "8.4"}},
				{ID: "semi-sync", Title: "Primary + semi-sync replica", Params: map[string]any{"version": "8.4", "replicas": 1, "replication": "semi_sync"}},
				{ID: "group-replication", Title: "Group Replication", Description: "Three members, single-primary, ProxySQL routing.", Params: map[string]any{"version": "8.4", "replicas": 2, "replication": "group", "proxysql": 1}},
			},
			ParamsSchema: "db.mysql.params@1", Protocols: []catalog.Protocol{catalog.ProtoMySQL}, Deployable: true,
		},
		plan:  planMySQL,
		login: func(map[string]any) topology.Login { return MySQLLogin },
	}
}

func mariadb() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.MariaDB, Title: "MariaDB", Description: "Single node, async/semi-sync replicas or Galera, ProxySQL or MaxScale in front.",
			Versions: []catalog.Version{
				{Version: "11.8", Image: "mariadb:11.8"},
				{Version: "11.4", Image: "mariadb:11.4", Default: true},
				{Version: "10.11", Image: "mariadb:10.11"},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Primary", Engine: "mariadb", ConfigSchemas: []string{"cfg.mariadb.cnf@11.4", "cfg.exporter.mysqld@1"}},
				{Role: "db-replica", Title: "Replica / Galera node", Engine: "mariadb", ConfigSchemas: []string{"cfg.mariadb.cnf@11.4"}},
				{Role: "proxy", Title: "ProxySQL / MaxScale", Engine: "proxysql", ConfigSchemas: []string{"cfg.proxysql.cnf@2", "cfg.maxscale.cnf@24"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "11.4"}},
				{ID: "galera", Title: "Galera cluster", Description: "Three multi-master nodes behind ProxySQL.", Params: map[string]any{"version": "11.4", "replication": "galera", "galera_nodes": 3, "proxysql": 1}},
			},
			ParamsSchema: "db.mariadb.params@1", Protocols: []catalog.Protocol{catalog.ProtoMySQL}, Deployable: true,
		},
		plan:  planMariaDB,
		login: func(map[string]any) topology.Login { return MySQLLogin },
	}
}

func planMySQL(p *topology.Plan, params map[string]any) {
	replicas := topology.IntOf(params, "replicas", 0)
	proxysql := topology.IntOf(params, "proxysql", 0)
	mode := topology.StrOf(params, "replication", "async")
	p.Add(topology.RoleDB, "mysql", 1)
	p.Add(topology.RoleDBReplica, "mysql", replicas)
	if replicas > 0 {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDBReplica, To: topology.RoleDB, Protocol: "mysql-replication", Port: 3306})
		if mode == "group" {
			for _, from := range []string{topology.RoleDB, topology.RoleDBReplica} {
				for _, to := range []string{topology.RoleDB, topology.RoleDBReplica} {
					p.Flows = append(p.Flows, topology.Flow{From: from, To: to, Protocol: "group-replication", Port: 33061},
						topology.Flow{From: from, To: to, Protocol: "group-recovery", Port: 3306})
				}
			}
		}
	}
	p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "mysql", Port: 3306}
	if proxysql > 0 {
		p.Add(topology.RoleProxy, "proxysql", proxysql)
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDB, Protocol: "mysql", Port: 3306})
		if replicas > 0 {
			p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDBReplica, Protocol: "mysql", Port: 3306})
		}
		p.Client = topology.Endpoint{Role: topology.RoleProxy, Protocol: "mysql", Port: 6033}
		p.Requirements[topology.RoleProxy] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "proxysql"}
	}
	req := topology.Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "mysql baseline"}
	p.Requirements[topology.RoleDB] = req
	if replicas > 0 {
		p.Requirements[topology.RoleDBReplica] = req
	}
	p.Label = topology.Label("mysql", replicas, "replica", mode == "group", "group replication", proxysql > 0, "proxysql", false, "")
}

func planMariaDB(p *topology.Plan, params map[string]any) {
	mode := topology.StrOf(params, "replication", "async")
	replicas := topology.IntOf(params, "replicas", 0)
	if mode == "galera" {
		nodes := topology.IntOf(params, "galera_nodes", 3)
		p.Add(topology.RoleDB, "mariadb", nodes)
		p.Flows = append(p.Flows,
			topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "galera", Port: 4567},
			topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "galera-ist", Port: 4568},
			topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "galera-sst", Port: 4444})
		replicas = 0
	} else {
		p.Add(topology.RoleDB, "mariadb", 1)
		p.Add(topology.RoleDBReplica, "mariadb", replicas)
		if replicas > 0 {
			p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDBReplica, To: topology.RoleDB, Protocol: "mysql-replication", Port: 3306})
		}
	}
	p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "mysql", Port: 3306}
	proxysql := topology.IntOf(params, "proxysql", 0)
	maxscale := topology.BoolOf(params, "maxscale")
	switch {
	case proxysql > 0:
		p.Add(topology.RoleProxy, "proxysql", proxysql)
		p.Client = topology.Endpoint{Role: topology.RoleProxy, Protocol: "mysql", Port: 6033}
	case maxscale:
		p.Add(topology.RoleProxy, "maxscale", 1)
		p.Client = topology.Endpoint{Role: topology.RoleProxy, Protocol: "mysql", Port: 4006}
	}
	if p.Client.Role == topology.RoleProxy {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDB, Protocol: "mysql", Port: 3306})
		if replicas > 0 {
			p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDBReplica, Protocol: "mysql", Port: 3306})
		}
		p.Requirements[topology.RoleProxy] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "proxy"}
	}
	req := topology.Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "mariadb baseline"}
	p.Requirements[topology.RoleDB] = req
	if replicas > 0 {
		p.Requirements[topology.RoleDBReplica] = req
	}
	if mode == "galera" {
		p.Label = fmt.Sprintf("galera ×%d", topology.IntOf(params, "galera_nodes", 3))
		if p.Client.Role == topology.RoleProxy {
			p.Label += " + " + p.Nodes[len(p.Nodes)-1].Engine
		}
		return
	}
	p.Label = topology.Label("mariadb", replicas, "replica", false, "", p.Client.Role == topology.RoleProxy, "proxy", false, "")
}
