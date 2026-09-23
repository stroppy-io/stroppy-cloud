package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// PostgreSQL family: the official image, PGDATA on the data disk, the
// rendered postgresql.conf / pg_hba.conf bind-mounted and passed with -c,
// the replication role and init SQL through docker-entrypoint-initdb.d.
// Replicas bootstrap with pg_basebackup -R before exec'ing the entrypoint.
//
// doc: hub.docker.com/_/postgres — "Database Configuration", "Initialization
// scripts"; postgresql.org/docs/current/app-pgbasebackup.html.

const (
	pgPort      = 5432
	pgDataInner = "/var/lib/postgresql/data"
	pgConfPath  = confDir + "/postgresql.conf"
	pgHBAPath   = confDir + "/pg_hba.conf"
	pgInitPath  = "/docker-entrypoint-initdb.d/00-stroppy.sql"
	pgbPort     = 6432
	proxyRW     = 5000
	proxyRO     = 5001
)

func postgresRecipe(c *compilation) error { return postgresFamily(c, false) }

func orioledbRecipe(c *compilation) error { return postgresFamily(c, true) }

func postgresFamily(c *compilation, oriole bool) error {
	p := c.params()
	if strParam(p, "ha", "none") == "patroni" {
		return patroniRecipe(c)
	}
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	confSchema := "cfg.postgresql.conf@"
	locale := strParam(p, "locale", "C")
	if oriole {
		confSchema = "cfg.orioledb.postgresql.conf@"
		locale = strParam(p, "initdb_locale", "C.UTF-8")
	}
	primary := c.first(topology.RoleDB)
	initSQL := pgInitSQL(strParam(p, "init_sql", ""), boolParam(p, "default_table_access_method"))

	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			conf, err := c.render(role, c.schemaFor(role, confSchema), "conf", map[string]any{"listen_addresses": "*", "port": pgPort})
			if err != nil {
				return err
			}
			hba, err := c.render(role, c.schemaFor(role, "cfg.pg_hba.conf@"), "conf", map[string]any{"rules": pgHBARules()})
			if err != nil {
				return err
			}
			ct := spec.Container{
				Name: m + "-" + c.engineOf(role), Role: role, Machine: m, Image: image,
				Kind: spec.ContainerKindDatabase,
				Env: map[string]string{
					"POSTGRES_USER": pgUser, "POSTGRES_PASSWORD": pgPassword, "POSTGRES_DB": pgDatabase, "PGDATA": pgDataInner,
					"POSTGRES_HOST_AUTH_METHOD": "scram-sha-256", "POSTGRES_INITDB_ARGS": "--data-checksums --locale=" + locale,
				},
				Mounts: []spec.Mount{{Source: pgDataDir, Target: pgDataInner}},
				Files: []spec.File{
					{Path: pgConfPath, Content: conf},
					{Path: pgHBAPath, Content: hba},
				},
				Ports:       []spec.Port{{Container: pgPort, Host: pgPort}},
				Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("pg_isready -h 127.0.0.1 -p %d -U %s", pgPort, pgUser)),
				Restart:     "always",
			}
			args := "postgres -c config_file=" + pgConfPath + " -c hba_file=" + pgHBAPath
			if role == topology.RoleDB {
				ct.Files = append(ct.Files, spec.File{Path: pgInitPath, Content: initSQL})
				ct.Cmd = strings.Fields(args)
			} else {
				ct.Cmd = []string{"bash", "-c", pgReplicaBootstrap(ip(primary), args)}
				ct.DependsOn = []string{primary + "-" + c.engineOf(topology.RoleDB)}
			}
			c.add(ct)
		}
	}
	c.postgresExporter(topology.RoleDB)
	c.postgresExporter(topology.RoleDBReplica)

	// Poolers and proxies.
	pgbouncer := boolParam(p, "pgbouncer")
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		listeners := []haproxyListener{{Name: "rw", BindPort: proxyRW, Servers: c.servers(topology.RoleDB, pgPort)}}
		if replicas := c.servers(topology.RoleDBReplica, pgPort); len(replicas) > 0 {
			listeners = append(listeners, haproxyListener{Name: "ro", BindPort: proxyRO, Servers: replicas})
		}
		if err := c.haproxy(listeners); err != nil {
			return err
		}
		if pgbouncer {
			return c.pgbouncer(topology.RoleProxy, "127.0.0.1", proxyRW)
		}
		return nil
	}
	if pgbouncer {
		return c.pgbouncer(topology.RoleDB, "127.0.0.1", pgPort)
	}
	return nil
}

// pgHBARules: local trust for the image entrypoint, password auth from
// the run network, replication for the replicas.
func pgHBARules() []any {
	rule := func(typ, db, user, addr, method string) map[string]any {
		m := map[string]any{"type": typ, "database": db, "user": user, "method": method}
		if addr != "" {
			m["address"] = addr
		}
		return m
	}
	return []any{
		rule("local", "all", "all", "", "trust"),
		rule("host", "all", "all", "127.0.0.1/32", "trust"),
		rule("host", "all", "all", "::1/128", "trust"),
		rule("host", "all", "all", "0.0.0.0/0", "scram-sha-256"),
		rule("host", "replication", replUser, "0.0.0.0/0", "scram-sha-256"),
	}
}

// pgInitSQL runs once on the primary's first start.
func pgInitSQL(extra string, orioleDefault bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE ROLE %s WITH REPLICATION LOGIN PASSWORD '%s';\n", replUser, replPassword)
	b.WriteString("CREATE EXTENSION IF NOT EXISTS pg_stat_statements;\n")
	if orioleDefault {
		b.WriteString("CREATE EXTENSION IF NOT EXISTS orioledb;\nALTER SYSTEM SET default_table_access_method = 'orioledb';\n")
	}
	if extra != "" {
		b.WriteString(strings.TrimSpace(extra))
		b.WriteString("\n")
	}
	return b.String()
}

// pgReplicaBootstrap clones the primary once, then runs the normal image
// entrypoint; -R writes primary_conninfo and standby.signal.
func pgReplicaBootstrap(primaryHost, args string) string {
	return fmt.Sprintf(`set -e
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  mkdir -p "$PGDATA" && chmod 700 "$PGDATA"
  n=0; until PGPASSWORD='%s' pg_basebackup -h %s -p %d -U %s -D "$PGDATA" -R -X stream -P; do
    n=$((n+1)); [ $n -ge 60 ] && exit 1; sleep 3; rm -rf "$PGDATA"/*; done
fi
exec docker-entrypoint.sh %s`, replPassword, primaryHost, pgPort, replUser, args)
}

// servers lists the machines of a role as haproxy backends.
func (c *compilation) servers(role string, port int) []haproxyServer {
	var out []haproxyServer
	for _, m := range c.machinesOf(role) {
		out = append(out, haproxyServer{Name: m, Address: ip(m), Port: port})
	}
	return out
}

// pgbouncer puts a pooler next to a role, forwarding to host:port.
func (c *compilation) pgbouncer(role, host string, port int) error {
	schemaID := c.schemaFor(role, "cfg.pgbouncer.ini@")
	if schemaID == "" {
		return errs.Newf(errs.CodeInvalid, "%s: role %s has no pgbouncer schema", c.in.Database.Kind, role)
	}
	ini, err := c.render(role, schemaID, "conf", map[string]any{
		"db_name": pgDatabase, "db_dbname": pgDatabase, "db_host": host, "db_port": port,
		"listen_addr": "*", "listen_port": pgbPort, "auth_type": "scram-sha-256", "auth_file": "/etc/pgbouncer/userlist.txt",
		"ignore_startup_parameters": "extra_float_digits",
	})
	if err != nil {
		return err
	}
	for _, m := range c.machinesOf(role) {
		deps := []string{}
		if role == topology.RoleDB {
			deps = append(deps, m+"-"+c.engineOf(role))
		} else {
			deps = append(deps, m+"-haproxy")
		}
		c.add(spec.Container{
			Name: m + "-pgbouncer", Role: role, Machine: m, Image: imagePgBouncer,
			Kind: spec.ContainerKindProxy,
			Files: []spec.File{
				{Path: "/etc/pgbouncer/pgbouncer.ini", Content: ini},
				{Path: "/etc/pgbouncer/userlist.txt", Content: fmt.Sprintf("%q %q\n", pgUser, pgPassword)},
			},
			Ports:       []spec.Port{{Container: pgbPort, Host: pgbPort}},
			Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("pg_isready -h 127.0.0.1 -p %d", pgbPort)),
			Restart:     "always",
			DependsOn:   deps,
		})
		c.add(spec.Container{
			Name: m + "-pgbouncer-exporter", Role: role, Machine: m, Image: imagePgBouncerExport,
			Kind:  spec.ContainerKindExporter,
			Cmd:   []string{"--pgBouncer.connectionString=" + fmt.Sprintf("postgresql://%s:%s@127.0.0.1:%d/pgbouncer?sslmode=disable", pgUser, pgPassword, pgbPort), "--web.listen-address=:9127"},
			Ports: []spec.Port{{Container: 9127, Host: 9127}}, Scrape: "/metrics",
			DependsOn: []string{m + "-pgbouncer"}, Restart: "always",
		})
	}
	return nil
}

// postgresExporter scrapes every postgres of a role from its own machine.
func (c *compilation) postgresExporter(role string) {
	for _, m := range c.machinesOf(role) {
		c.add(spec.Container{
			Name: m + "-postgres-exporter", Role: role, Machine: m, Image: imagePostgresExport,
			Kind:      spec.ContainerKindExporter,
			Env:       map[string]string{"DATA_SOURCE_NAME": fmt.Sprintf("postgresql://%s:%s@127.0.0.1:%d/%s?sslmode=disable", pgUser, pgPassword, pgPort, pgDatabase)},
			Cmd:       []string{"--collector.postmaster", "--collector.stat_statements", fmt.Sprintf("--web.listen-address=:%d", postgresExporterPort)},
			Ports:     []spec.Port{{Container: postgresExporterPort, Host: postgresExporterPort}},
			Scrape:    "/metrics",
			Restart:   "always",
			DependsOn: []string{m + "-" + c.engineOf(role)},
		})
	}
}

// pgNoopRecipe: a protocol-only server, no storage.
func pgNoopRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	p := c.params()
	if numberOf(p["latency_ms"]) != 0 || numberOf(p["error_rate"]) != 0 {
		return fmt.Errorf("pg-noop 0.1.2 does not implement latency or error injection")
	}
	port := intParam(p, "port", pgPort)
	for _, m := range c.machinesOf(topology.RoleDB) {
		env := map[string]string{"PGNOOP_HOST": "0.0.0.0", "PGNOOP_PORT": fmt.Sprint(port)}
		// Zero means automatic sizing in the catalog. pg-noop's runtime
		// rejects a literal zero, so let its default choose the worker count.
		if workers := intParam(p, "workers", 0); workers > 0 {
			env["PGNOOP_WORKERS"] = fmt.Sprint(workers)
		}
		c.add(spec.Container{
			Name: m + "-pg-noop", Role: topology.RoleDB, Machine: m, Image: image,
			Kind:        spec.ContainerKindDatabase,
			Env:         env,
			Ports:       []spec.Port{{Container: port, Host: port}},
			Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("nc -z 127.0.0.1 %d", port)),
			Restart:     "always",
		})
	}
	return nil
}

// noopRecipe: nothing to deploy (noop, external, managed).
func noopRecipe(*compilation) error { return nil }

var _ = catalog.Noop
