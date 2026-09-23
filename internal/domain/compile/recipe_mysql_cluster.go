package compile

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

//go:embed mysql-group-start.sh
var mysqlGroupStart string

//go:embed mariadb-galera-start.sh
var mariadbGaleraStart string

const (
	clusterStartPath = "/usr/local/bin/stroppy-cluster-start.sh"
	recoveryPassword = "stroppy_recovery"
)

// mysqlClusterRecipe starts a fresh group once, then joins subsequent nodes.
// Initial readiness follows the bootstrap order; the proxy checks all members
// before accepting workload traffic. A persisted marker forbids auto-bootstrap
// on restart. Full-cluster disaster recovery is deliberately not automatic.
func mysqlClusterRecipe(c *compilation, maria bool) error {
	if boolParam(c.params(), "maxscale") {
		return errs.Newf(errs.CodeInvalid, "params.maxscale: maxscale is not launchable yet")
	}
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	members := append(c.machinesOf(topology.RoleDB), c.machinesOf(topology.RoleDBReplica)...)
	peers := make([]string, len(members))
	for i, m := range members {
		peers[i] = ip(m)
	}
	previous := ""
	for i, m := range members {
		role := topology.RoleDB
		if !maria && i > 0 {
			role = topology.RoleDBReplica
		}
		overlay := map[string]any{
			"server_id": i + 1, "report_host": ip(m), "bind_address": "0.0.0.0", "port": mysqlPort,
			"binlog_format": "ROW", "read_only": "OFF",
		}
		prefix, startup := "cfg.my.cnf@", mysqlGroupStart
		if maria {
			prefix, startup = "cfg.mariadb.cnf@", mariadbGaleraStart
			overlay["wsrep_on"] = "ON"
			overlay["innodb_autoinc_lock_mode"] = 2
			overlay["wsrep_cluster_name"] = strings.ReplaceAll(c.in.RunID.String(), "-", "")
			overlay["wsrep_cluster_address"] = "gcomm://" + strings.Join(peers, ",")
			overlay["wsrep_node_address"], overlay["wsrep_node_name"] = ip(m), m
			overlay["wsrep_sst_auth"] = "root:" + mysqlPassword
		} else {
			seeds := make([]string, len(peers))
			for j, peer := range peers {
				seeds[j] = peer + ":33061"
			}
			overlay["gtid_mode"], overlay["enforce_gtid_consistency"] = "ON", "ON"
			overlay["super_read_only"] = "OFF"
			overlay["group_replication"] = true
			overlay["group_replication_group_name"] = c.in.RunID.String()
			overlay["group_replication_local_address"] = ip(m) + ":33061"
			overlay["group_replication_group_seeds"] = strings.Join(seeds, ",")
			overlay["group_replication_bootstrap_group"], overlay["group_replication_start_on_boot"] = "OFF", "OFF"
			overlay["group_replication_single_primary_mode"], overlay["group_replication_enforce_update_everywhere_checks"] = "ON", "OFF"
			if !boolParam(c.params(), "single_primary") {
				overlay["group_replication_single_primary_mode"], overlay["group_replication_enforce_update_everywhere_checks"] = "OFF", "ON"
			}
		}
		conf, err := c.render(role, c.schemaFor(role, prefix), "conf", overlay)
		if err != nil {
			return err
		}
		if !maria {
			// Topology-owned settings, common to both supported MySQL series.
			// doc: dev.mysql.com/doc/refman/8.4/en/group-replication-system-variables.html
			conf += "\nloose-group_replication_communication_stack = XCOM\nloose-group_replication_ssl_mode = REQUIRED\nloose-group_replication_recovery_use_ssl = ON\nloose-group_replication_ip_allowlist = " + strings.Join(peers, ",") + "\n"
		}
		ct := spec.Container{
			Name: m + "-" + c.engineOf(role), Role: role, Machine: m, Image: image,
			Kind:   spec.ContainerKindDatabase,
			Cmd:    []string{"bash", clusterStartPath},
			Env:    map[string]string{"MYSQL_ROOT_PASSWORD": mysqlPassword, "MYSQL_ROOT_HOST": "%", "MARIADB_ROOT_PASSWORD": mysqlPassword, "MARIADB_ROOT_HOST": "%", "STROPPY_CLUSTER_SEED": "0", "STROPPY_RECOVERY_PASSWORD": recoveryPassword},
			Mounts: []spec.Mount{{Source: dataMount + "/mysql", Target: mysqlDataInner}},
			Files:  []spec.File{{Path: mysqlConfPath, Content: conf}, {Path: clusterStartPath, Content: startup, Mode: "0755"}},
			Ports:  []spec.Port{{Container: mysqlPort, Host: mysqlPort}}, Restart: "always",
		}
		init := ""
		if !maria {
			// Timezone initialization creates local GTIDs; only the seed loads it.
			if i > 0 {
				ct.Env["MYSQL_INITDB_SKIP_TZINFO"] = "1"
			}
			// Recovery users are local bootstrap credentials, not divergent GTIDs.
			// doc: dev.mysql.com/doc/refman/8.4/en/group-replication-user-credentials.html
			init = fmt.Sprintf("SET SESSION SQL_LOG_BIN=0;\nCREATE USER 'stroppy_recovery'@'%%' IDENTIFIED BY '%s';\nGRANT REPLICATION SLAVE, CONNECTION_ADMIN ON *.* TO 'stroppy_recovery'@'%%';\nSET SESSION SQL_LOG_BIN=1;\n", recoveryPassword)
			ct.Ports = append(ct.Ports, spec.Port{Container: 33061, Host: 33061})
		} else {
			for _, port := range []int{4567, 4568, 4444} {
				ct.Ports = append(ct.Ports, spec.Port{Container: port, Host: port})
			}
		}
		if i == 0 {
			ct.Env["STROPPY_CLUSTER_SEED"] = "1"
			ct.Env["MYSQL_DATABASE"], ct.Env["MARIADB_DATABASE"] = mysqlDatabase, mysqlDatabase
			init += mysqlInitSQL(maria, strParam(c.params(), "init_sql", ""))
		}
		if init != "" {
			ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: init})
		}
		if previous != "" {
			ct.DependsOn = []string{previous}
		}
		ct.Healthcheck = healthcheck("CMD-SHELL", clusterSQLCheck(maria, "127.0.0.1", i+1, false))
		c.add(ct)
		previous = ct.Name
		cmd := []string{fmt.Sprintf("--mysqld.address=127.0.0.1:%d", mysqlPort), "--mysqld.username=" + exporterUser, fmt.Sprintf("--web.listen-address=:%d", mysqldExporterPort)}
		if !maria {
			cmd = append(cmd, "--collect.perf_schema.replication_group_members", "--collect.perf_schema.replication_group_member_stats")
		}
		c.add(spec.Container{
			Name: m + "-mysqld-exporter", Role: role, Machine: m, Image: imageMySQLDExporter,
			Kind: spec.ContainerKindExporter,
			Env:  map[string]string{"MYSQLD_EXPORTER_PASSWORD": exporterPassword}, Cmd: cmd,
			Ports: []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}}, Scrape: "/metrics", Restart: "always", DependsOn: []string{ct.Name},
		})
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.proxysql()
	}
	return nil
}

func clusterSQLCheck(maria bool, host string, members int, proxyClient bool) string {
	query := fmt.Sprintf("SELECT COUNT(*) >= %d AND SUM(MEMBER_STATE='ONLINE')=COUNT(*) FROM performance_schema.replication_group_members", members)
	client, tls := "mysql", "--ssl-mode=REQUIRED"
	if proxyClient {
		tls = "--ssl --ssl-verify-server-cert=0"
	}
	if maria {
		client, tls = "mariadb", "--skip-ssl"
		query = fmt.Sprintf("SELECT (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_CLUSTER_SIZE') >= %d AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_LOCAL_STATE')='4' AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_READY')='ON' AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_CLUSTER_STATUS')='Primary'", members)
	}
	return fmt.Sprintf("test \"$(%s %s --connect-timeout=3 -h%s -uroot -p%s -Nse %q)\" = 1", client, tls, host, mysqlPassword, query)
}
