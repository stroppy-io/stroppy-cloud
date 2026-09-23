package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// MySQL / MariaDB: the official images, datadir on the data disk, the
// rendered my.cnf dropped into conf.d, root@% with the well-known password
// through the image env, replicas configured by an initdb.d script.
//
// doc: hub.docker.com/_/mysql, hub.docker.com/_/mariadb — env, conf.d,
// docker-entrypoint-initdb.d; dev.mysql.com/doc/refman/8.4/en/
// change-replication-source-to.html; mariadb.com/kb/en/change-master-to.

const (
	mysqlPort        = 3306
	mysqlDataInner   = "/var/lib/mysql"
	mysqlConfPath    = "/etc/mysql/conf.d/zz-stroppy.cnf"
	mysqlInitPath    = "/docker-entrypoint-initdb.d/10-stroppy.sql"
	proxysqlPort     = 6033
	proxysqlConfPath = "/etc/proxysql.cnf"
	exporterUser     = "exporter"
	exporterPassword = "stroppy_exporter"
)

func mysqlRecipe(c *compilation) error {
	p := c.params()
	maria := c.in.Database.Kind == catalog.MariaDB
	mode := strParam(p, "replication", "async")
	switch {
	case mode == "group":
		return mysqlClusterRecipe(c, false)
	case mode == "galera":
		return mysqlClusterRecipe(c, true)
	case boolParam(p, "maxscale"):
		return errs.Newf(errs.CodeInvalid, "params.maxscale: maxscale is not launchable yet")
	}
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	confPrefix := "cfg.my.cnf@"
	if maria {
		confPrefix = "cfg.mariadb.cnf@"
	}
	primary := c.first(topology.RoleDB)
	serverID := 0
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			serverID++
			overlay := map[string]any{"server_id": serverID, "report_host": ip(m), "bind_address": "0.0.0.0", "port": mysqlPort}
			if role == topology.RoleDBReplica {
				overlay["read_only"] = "ON"
			}
			if !maria {
				overlay["gtid_mode"] = "ON"
				overlay["enforce_gtid_consistency"] = "ON"
				if mode == "semi_sync" {
					overlay["rpl_semi_sync_source_enabled"] = "OFF"
					overlay["rpl_semi_sync_replica_enabled"] = "OFF"
					if role == topology.RoleDB {
						overlay["rpl_semi_sync_source_enabled"] = "ON"
					} else {
						overlay["rpl_semi_sync_replica_enabled"] = "ON"
					}
				}
			}
			conf, err := c.render(role, c.schemaFor(role, confPrefix), "conf", overlay)
			if err != nil {
				return err
			}
			if !maria && mode == "semi_sync" && role == topology.RoleDB {
				// The database schema validates the requested ack count against
				// the number of replicas. Keep it in the startup configuration.
				conf += fmt.Sprintf("\nloose-rpl_semi_sync_source_wait_for_replica_count = %d\n",
					int(numberOf(p["semi_sync_wait_for_slave_count"])))
			}
			ct := spec.Container{
				Name: m + "-" + c.engineOf(role), Role: role, Machine: m, Image: image,
				Kind:   spec.ContainerKindDatabase,
				Env:    map[string]string{"MYSQL_ROOT_PASSWORD": mysqlPassword, "MYSQL_ROOT_HOST": "%", "MARIADB_ROOT_PASSWORD": mysqlPassword, "MARIADB_ROOT_HOST": "%"},
				Mounts: []spec.Mount{{Source: dataMount + "/mysql", Target: mysqlDataInner}},
				Files:  []spec.File{{Path: mysqlConfPath, Content: conf}},
				Ports:  []spec.Port{{Container: mysqlPort, Host: mysqlPort}},
				Healthcheck: healthcheck("CMD-SHELL",
					fmt.Sprintf("%s -h127.0.0.1 -uroot -p%s -e 'SELECT 1'", clientOf(maria), mysqlPassword)),
				Restart: "always",
			}
			if !maria && mode == "semi_sync" && role == topology.RoleDB {
				// Fail readiness if the plugin was not loaded after initialization.
				ct.Healthcheck = healthcheck("CMD-SHELL", fmt.Sprintf(
					"test \"$(mysql -h127.0.0.1 -uroot -p%s -Nse 'SELECT @@GLOBAL.rpl_semi_sync_source_enabled')\" = 1", mysqlPassword))
			}
			if role == topology.RoleDB {
				ct.Env["MYSQL_DATABASE"], ct.Env["MARIADB_DATABASE"] = mysqlDatabase, mysqlDatabase
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: mysqlInitSQL(maria, strParam(p, "init_sql", ""))})
			} else {
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: mysqlReplicaSQL(maria, ip(primary))})
				ct.DependsOn = []string{primary + "-" + c.engineOf(topology.RoleDB)}
				if !maria {
					ct.Healthcheck = healthcheck("CMD-SHELL", mysqlReplicaHealth(mode == "semi_sync"))
				}
			}
			c.add(ct)
		}
	}
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			c.add(spec.Container{
				Name: m + "-mysqld-exporter", Role: role, Machine: m, Image: imageMySQLDExporter,
				Kind: spec.ContainerKindExporter,
				Env:  map[string]string{"MYSQLD_EXPORTER_PASSWORD": exporterPassword},
				Cmd: []string{
					fmt.Sprintf("--mysqld.address=127.0.0.1:%d", mysqlPort), "--mysqld.username=" + exporterUser, fmt.Sprintf("--web.listen-address=:%d", mysqldExporterPort),
				},
				Ports:     []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}},
				Scrape:    "/metrics",
				Restart:   "always",
				DependsOn: []string{m + "-" + c.engineOf(role)},
			})
		}
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.proxysql()
	}
	return nil
}

func clientOf(maria bool) string {
	if maria {
		return "mariadb"
	}
	return "mysql"
}

// A listening replica is not necessarily replicating. Wait for both the
// receiver and applier and for the primary's initial database to arrive.
func mysqlReplicaHealth(semisync bool) string {
	query := "SELECT @@GLOBAL.read_only = 1 AND " +
		"EXISTS(SELECT 1 FROM performance_schema.replication_connection_status WHERE CHANNEL_NAME='' AND SERVICE_STATE='ON') AND " +
		"EXISTS(SELECT 1 FROM performance_schema.replication_applier_status WHERE CHANNEL_NAME='' AND SERVICE_STATE='ON') AND " +
		"EXISTS(SELECT 1 FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='stroppy')"
	if semisync {
		query += " AND @@GLOBAL.rpl_semi_sync_replica_enabled = 1 AND " +
			"EXISTS(SELECT 1 FROM performance_schema.global_status WHERE VARIABLE_NAME='Rpl_semi_sync_replica_status' AND VARIABLE_VALUE='ON')"
	}
	return fmt.Sprintf("test \"$(mysql -h127.0.0.1 -uroot -p%s -Nse %q)\" = 1", mysqlPassword, query)
}

// mysqlInitSQL runs once on the primary: the exporter user and init SQL.
func mysqlInitSQL(maria bool, extra string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';\n", exporterUser, exporterPassword)
	fmt.Fprintf(&b, "GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO '%s'@'%%';\n", exporterUser)
	if maria {
		fmt.Fprintf(&b, "GRANT SLAVE MONITOR ON *.* TO '%s'@'%%';\n", exporterUser)
	}
	b.WriteString("FLUSH PRIVILEGES;\n")
	if extra != "" {
		b.WriteString(strings.TrimSpace(extra))
		b.WriteString("\n")
	}
	return b.String()
}

// mysqlReplicaSQL points a fresh replica at the primary; GTID auto
// positioning replays everything the primary did, the stroppy database
// included.
func mysqlReplicaSQL(maria bool, primaryHost string) string {
	if maria {
		return fmt.Sprintf("CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='root', MASTER_PASSWORD='%s', MASTER_USE_GTID=slave_pos;\nSTART SLAVE;\nSET GLOBAL read_only = ON;\n",
			primaryHost, mysqlPort, mysqlPassword)
	}
	return fmt.Sprintf("CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='root', SOURCE_PASSWORD='%s', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1;\nSTART REPLICA;\nSET GLOBAL read_only = ON;\n",
		primaryHost, mysqlPort, mysqlPassword)
}

// proxysql renders proxysql.cnf with the primary in the writer hostgroup
// and the replicas in the reader hostgroup.
func (c *compilation) proxysql() error {
	role := topology.RoleProxy
	schemaID := c.schemaFor(role, "cfg.proxysql.cnf@")
	if schemaID == "" {
		return errs.Newf(errs.CodeInvalid, "%s: role %s has no proxysql schema", c.in.Database.Kind, role)
	}
	eff := c.effective(role, schemaID)
	writer, reader := int(numberOf(eff["writer_hostgroup"])), int(numberOf(eff["reader_hostgroup"]))
	if writer == 0 && reader == 0 {
		writer, reader = 10, 20
	}
	maria := c.in.Database.Kind == catalog.MariaDB
	mode := strParam(c.params(), "replication", "async")
	members := append(c.machinesOf(topology.RoleDB), c.machinesOf(topology.RoleDBReplica)...)
	servers := []any{}
	dependencies := []string{}
	for _, dbRole := range []string{topology.RoleDB, topology.RoleDBReplica} {
		hostgroup := writer
		if dbRole == topology.RoleDBReplica {
			hostgroup = reader
		}
		for _, m := range c.machinesOf(dbRole) {
			server := map[string]any{"address": ip(m), "port": mysqlPort, "hostgroup": hostgroup}
			if !maria {
				server["use_ssl"] = 1
			}
			servers = append(servers, server)
			dependencies = append(dependencies, m+"-"+c.engineOf(dbRole))
		}
	}
	overlay := map[string]any{
		"mysql_servers": servers, "username": mysqlUser, "password": mysqlPassword,
		"interfaces": fmt.Sprintf("0.0.0.0:%d", proxysqlPort), "monitor_username": exporterUser, "monitor_password": exporterPassword,
		"restapi_enabled": "true", "restapi_port": 6070, "admin_mysql_ifaces": "127.0.0.1:6032",
	}
	if mode == "group" {
		overlay["topology"] = "group_replication"
	}
	if mode == "galera" {
		overlay["topology"] = "galera"
	}
	cnf, err := c.render(role, schemaID, "conf", overlay)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("test \"$(mysql --skip-ssl --connect-timeout=3 -h127.0.0.1 -P%d -u%s -p%s -D%s -Nse 'SELECT 1')\" = 1", proxysqlPort, mysqlUser, mysqlPassword, mysqlDatabase)
	if mode == "group" || mode == "galera" {
		for _, m := range members {
			query += " && " + clusterSQLCheck(maria, ip(m), len(members), true)
		}
	}
	for _, m := range c.machinesOf(role) {
		c.add(spec.Container{
			Name: m + "-proxysql", Role: role, Machine: m, Image: imageProxySQL,
			Kind: spec.ContainerKindProxy,
			// Raise only this process's soft limit within the existing hard limit.
			Cmd:   []string{"bash", "-ec", `ulimit -Sn "$(ulimit -Hn)"; exec proxysql -f --idle-threads -D /var/lib/proxysql`},
			Files: []spec.File{{Path: proxysqlConfPath, Content: cnf}, {Path: "/etc/stroppy/proxy-health.sh", Content: "#!/usr/bin/env bash\nset -euo pipefail\n" + query + "\n", Mode: "0755"}},
			// The observer scrapes the first declared port.
			Ports: []spec.Port{{Container: 6070, Host: 6070}, {Container: proxysqlPort, Host: proxysqlPort}}, Scrape: "/metrics",
			Healthcheck: healthcheck("CMD", "bash", "/etc/stroppy/proxy-health.sh"), Restart: "always", DependsOn: dependencies,
		})
	}
	return nil
}
