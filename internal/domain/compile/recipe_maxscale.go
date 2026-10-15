package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// MariaDB MaxScale in front of a MariaDB primary with replicas (mariadbmon)
// or a Galera cluster (galeramon): one readwritesplit service, one listener
// the workload connects to. The image's own start (monit, logs to files) is
// replaced by MaxScale in the foreground logging to stdout.
//
// LICENSE: MaxScale is under the Business Source License — production use
// is free only with fewer than three database servers. The stand is the
// user's own and the parameter's description says so (cfg/maxscale.go).
//
// doc: mariadb.com/docs/maxscale/maxscale-management/deployment/installation-and-configuration/maxscale-configuration-guide
// — the service and monitor accounts and their grants.

const (
	maxscaleConfPath = "/etc/maxscale.cnf"
	maxscalePassword = "stroppy_maxscale" //nolint:gosec // G101: fixed benchmark-stand password, not a credential
)

// maxscaleUsersSQL creates the accounts MaxScale logs in with. It runs on
// the primary (or the first Galera node) and replicates to the others.
func maxscaleUsersSQL() string {
	var b strings.Builder
	for _, u := range []string{"maxscale_service", "maxscale_monitor", "maxscale_repl"} {
		fmt.Fprintf(&b, "CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';\n", u, maxscalePassword)
	}
	for _, table := range []string{"user", "db", "tables_priv", "columns_priv", "procs_priv", "proxies_priv", "roles_mapping"} {
		fmt.Fprintf(&b, "GRANT SELECT ON mysql.%s TO 'maxscale_service'@'%%';\n", table)
	}
	b.WriteString("GRANT SHOW DATABASES ON *.* TO 'maxscale_service'@'%';\n")
	b.WriteString("GRANT PROCESS, REPLICATION CLIENT, SLAVE MONITOR ON *.* TO 'maxscale_monitor'@'%';\n")
	b.WriteString("GRANT REPLICATION SLAVE ON *.* TO 'maxscale_repl'@'%';\n")
	return b.String()
}

// maxscale renders maxscale.cnf over the database machines and adds the
// proxy container; galera picks galeramon.
func (c *compilation) maxscale(galera bool) error {
	role := topology.RoleProxy
	schemaID := c.schemaFor(role, "cfg.maxscale.cnf@")
	if schemaID == "" {
		return errs.Newf(errs.CodeInvalid, "%s: role %s has no maxscale schema", c.in.Database.Kind, role)
	}
	servers, dependencies := []any{}, []string{}
	members := 0
	for _, dbRole := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(dbRole) {
			servers = append(servers, map[string]any{"name": m, "address": ip(m), "port": mysqlPort})
			dependencies = append(dependencies, m+"-"+c.engineOf(dbRole))
			members++
		}
	}
	module := "mariadbmon"
	if galera {
		module = "galeramon"
	}
	eff := c.effective(role, schemaID)
	port := int(numberOf(eff["listener_port"]))
	if port == 0 {
		port = 4006
	}
	overlay := map[string]any{
		"servers": servers, "monitor_module": module,
		"monitor_password": maxscalePassword, "service_password": maxscalePassword, "replication_password": maxscalePassword,
		"admin_host": "127.0.0.1", "admin_secure_gui": "false",
		// The workload logs in as root, which MaxScale refuses by default.
		"enable_root_user": "true",
	}
	cnf, err := c.render(role, schemaID, "conf", overlay)
	if err != nil {
		return err
	}
	// Ready once the monitor sees a primary and every backend running.
	health := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\nstates=\"$(maxctrl --tsv list servers | cut -f5)\"\n"+
		"grep -q Master <<<\"$states\"\ntest \"$(grep -c Running <<<\"$states\")\" -ge %d\n", members)
	for _, m := range c.machinesOf(role) {
		c.add(spec.Container{
			Name: m + "-maxscale", Role: role, Machine: m, Image: imageMaxScale,
			Kind:        spec.ContainerKindProxy,
			Cmd:         []string{"maxscale", "-d", "-U", "maxscale", "-l", "stdout"},
			Files:       []spec.File{{Path: maxscaleConfPath, Content: cnf}, {Path: "/etc/stroppy/proxy-health.sh", Content: health, Mode: "0755"}},
			Ports:       []spec.Port{{Container: port, Host: port}},
			Healthcheck: healthcheck("CMD", "bash", "/etc/stroppy/proxy-health.sh"), Restart: "always", DependsOn: dependencies,
		})
	}
	return nil
}
