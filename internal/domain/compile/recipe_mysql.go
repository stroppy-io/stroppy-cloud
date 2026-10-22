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
	switch mode {
	case "group":
		return mysqlClusterRecipe(c, false)
	case "galera":
		return mysqlClusterRecipe(c, true)
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
			source, replica := "rpl_semi_sync_source_enabled", "rpl_semi_sync_replica_enabled"
			if maria {
				// MariaDB has semisync built in (10.3.3+) under the
				// master/slave names; nothing else turns it on.
				// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication
				source, replica = "rpl_semi_sync_master_enabled", "rpl_semi_sync_slave_enabled"
			} else {
				overlay["gtid_mode"] = "ON"
				overlay["enforce_gtid_consistency"] = "ON"
			}
			if mode == "semi_sync" {
				overlay[source], overlay[replica] = "OFF", "OFF"
				if role == topology.RoleDB {
					overlay[source] = "ON"
				} else {
					overlay[replica] = "ON"
				}
			}
			schemaID := c.schemaFor(role, confPrefix)
			c.tuneInnoDB(innodbServer{role: role, schemaID: schemaID, machine: m}, overlay)
			conf, err := c.render(role, schemaID, "conf", overlay)
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
				init := mysqlInitSQL(maria, strParam(p, "init_sql", ""))
				if boolParam(p, "maxscale") {
					init += maxscaleUsersSQL()
				}
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: init})
			} else {
				replicaSQL := mysqlReplicaSQL(maria, ip(primary))
				if !maria && c.innodbAuto() && !c.explicitConfig(role, schemaID)["super_read_only"] {
					// super_read_only in my.cnf would also bind the image's
					// temporary init server and fail its CREATE USER; a
					// persisted value applies from the real server's start.
					// doc: https://dev.mysql.com/doc/refman/8.4/en/persisted-system-variables.html
					replicaSQL += "SET PERSIST super_read_only = ON;\n"
					c.recordTuned(role, schemaID, "super_read_only", "ON")
				}
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: replicaSQL})
				ct.DependsOn = []string{primary + "-" + c.engineOf(topology.RoleDB)}
				if !maria {
					ct.Healthcheck = healthcheck("CMD-SHELL", mysqlReplicaHealth(mode == "semi_sync"))
				}
			}
			c.mariadbIOUring(&ct)
			c.add(ct)
		}
	}
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			c.add(spec.Container{
				Name: m + "-mysqld-exporter", Role: role, Machine: m, Image: imageMySQLDExporter,
				Kind:      spec.ContainerKindExporter,
				Env:       map[string]string{"MYSQLD_EXPORTER_PASSWORD": exporterPassword},
				Cmd:       mysqldExporterCmd(fmt.Sprintf("127.0.0.1:%d", mysqlPort), exporterUser, false),
				Ports:     []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}},
				Scrape:    "/metrics",
				Restart:   "always",
				DependsOn: []string{m + "-" + c.engineOf(role)},
			})
		}
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		if boolParam(p, "maxscale") {
			return c.maxscale(false)
		}
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
	// slave_hosts reads SHOW REPLICAS: REPLICATION SLAVE on MySQL,
	// REPLICATION MASTER ADMIN on MariaDB 10.5+.
	fmt.Fprintf(&b, "GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO '%s'@'%%';\n", exporterUser)
	if maria {
		fmt.Fprintf(&b, "GRANT SLAVE MONITOR, REPLICATION MASTER ADMIN ON *.* TO '%s'@'%%';\n", exporterUser)
	} else {
		fmt.Fprintf(&b, "GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';\n", exporterUser)
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
	auto := c.innodbAuto()
	confPrefix := "cfg.my.cnf@"
	if maria {
		confPrefix = "cfg.mariadb.cnf@"
	}
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
			if auto {
				// The pool may open what the backend accepts, less room for
				// the exporter, the monitor and an operator.
				if limit := int(numberOf(c.configValue(dbRole, c.schemaFor(dbRole, confPrefix), "max_connections"))); limit > 100 {
					server["max_connections"] = limit - 50
				}
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
	if maria && !c.explicitConfig(role, schemaID)["server_version"] {
		// The handshake version drivers see; the image tag floats within
		// the series, so the patch level is not known here.
		// doc: https://proxysql.com/documentation/global-variables/mysql-variables/#mysql-server_version
		overlay["server_version"] = c.in.Database.Version + ".0-MariaDB"
	}
	if auto {
		t := c.tune(role, schemaID, overlay)
		// One worker per proxy core (ProxySQL's guidance; fixed at start).
		// doc: https://proxysql.com/documentation/global-variables/mysql-variables/#mysql-threads
		if machines := c.machinesOf(role); len(machines) > 0 {
			if cpu := c.hostOf(machines[0]).CPU; cpu > 0 {
				t.set("threads", clamp(cpu, 1, 16))
			}
		}
		t.set("max_connections", max(2048, 2*c.peakVUs()))
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
