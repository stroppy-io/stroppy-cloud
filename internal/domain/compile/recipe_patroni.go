package compile

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const (
	patroniPort = 8008
	patroniPath = confDir + "/patroni.yml"
	patroniInit = confDir + "/patroni-init.sh"
	imageEtcd   = "quay.io/coreos/etcd:v3.5.33"
)

func patroniRecipe(c *compilation) error {
	if len(c.machinesOf(topology.RoleProxy)) == 0 {
		return errs.Newf(errs.CodeInvalid, "params.haproxy: Patroni requires HAProxy to route workload connections to the elected leader")
	}
	if c.in.Database.Image != "" {
		return errs.Newf(errs.CodeInvalid, "database.image: Patroni uses the versioned Patroni/PostgreSQL image; a plain PostgreSQL image cannot run Patroni")
	}
	etcdNames, hosts, err := c.patroniEtcd()
	if err != nil {
		return err
	}
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			if err := c.patroniNode(role, m, hosts, etcdNames); err != nil {
				return err
			}
		}
		c.postgresExporter(role)
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		servers := append(c.servers(topology.RoleDB, pgPort), c.servers(topology.RoleDBReplica, pgPort)...)
		if err := c.haproxy([]haproxyListener{
			{Name: "rw", BindPort: proxyRW, Servers: servers, Check: map[string]any{"kind": "httpchk", "uri": "/primary", "port": patroniPort}},
			{Name: "ro", BindPort: proxyRO, Servers: servers, Check: map[string]any{"kind": "httpchk", "uri": "/replica", "port": patroniPort}},
		}); err != nil {
			return err
		}
		if boolParam(c.params(), "pgbouncer") {
			return c.pgbouncer(topology.RoleProxy, "127.0.0.1", proxyRW)
		}
	}
	return nil
}

func (c *compilation) patroniEtcd() (names []string, hosts []any, err error) {
	var members []string
	for _, m := range c.machinesOf(topology.RoleEtcd) {
		names = append(names, m+"-etcd")
		members = append(members, m+"=http://"+ip(m)+":2380")
		hosts = append(hosts, ip(m)+":2379")
	}
	for _, m := range c.machinesOf(topology.RoleEtcd) {
		cfg, err := c.render(topology.RoleEtcd, "cfg.etcd@3", "conf", map[string]any{
			"name": m, "data_dir": "/var/lib/etcd", "initial_cluster": members,
			"initial_cluster_token": c.in.RunID.String(), "initial_cluster_state": "new",
			"initial_advertise_peer_urls": "http://" + ip(m) + ":2380", "advertise_client_urls": "http://" + ip(m) + ":2379",
			"listen_peer_urls": "http://0.0.0.0:2380", "listen_client_urls": "http://0.0.0.0:2379",
		})
		if err != nil {
			return nil, nil, err
		}
		c.add(spec.Container{
			Name: m + "-etcd", Role: topology.RoleEtcd, Machine: m, Image: imageEtcd,
			Kind:        spec.ContainerKindCoordinator,
			Cmd:         []string{"/usr/local/bin/etcd", "--config-file=" + confDir + "/etcd.yml"},
			Files:       []spec.File{{Path: confDir + "/etcd.yml", Content: cfg}},
			Mounts:      []spec.Mount{{Source: dataMount + "/etcd", Target: "/var/lib/etcd"}},
			Ports:       []spec.Port{{Container: 2379, Host: 2379}, {Container: 2380, Host: 2380}},
			Scrape:      "/metrics",
			Healthcheck: healthcheck("CMD", "/usr/local/bin/etcdctl", "--endpoints=http://127.0.0.1:2379", "endpoint", "health"), Restart: "always",
		})
	}
	return names, hosts, nil
}

func (c *compilation) patroniNode(role, m string, hosts []any, deps []string) error {
	c.out.Spec.Scrapes = append(c.out.Spec.Scrapes, spec.Scrape{Role: role, Job: m + "-" + c.engineOf(role), URL: "http://127.0.0.1:8008/metrics"})
	conf, err := c.render(role, c.schemaFor(role, "cfg.postgresql.conf@"), "conf", map[string]any{"listen_addresses": "*", "port": pgPort})
	if err != nil {
		return err
	}
	hba, err := c.render(role, "cfg.pg_hba.conf@1", "conf", map[string]any{"rules": pgHBARules()})
	if err != nil {
		return err
	}
	cfg, err := c.patroniConfig(role, m, hosts)
	if err != nil {
		return err
	}
	// Patroni creates its superuser and replication roles. Its post-bootstrap
	// hook creates the workload database and extensions once, before cloning.
	init := fmt.Sprintf("#!/bin/sh\nset -eu\npsql \"$1\" -v ON_ERROR_STOP=1 <<'STROPPY_SQL'\nCREATE EXTENSION IF NOT EXISTS pg_stat_statements;\n%s\nSTROPPY_SQL\n", strings.TrimSpace(strParam(c.params(), "init_sql", "")))
	c.add(spec.Container{
		Name: m + "-" + c.engineOf(role), Role: role, Machine: m,
		Kind:        spec.ContainerKindDatabase,
		Image:       "docker.stroppy.io/stroppy-io/patroni:pg" + c.in.Database.Version + "-4.1.5",
		Env:         map[string]string{"PGDATA": pgDataInner + "/pgdata"},
		Mounts:      []spec.Mount{{Source: pgDataDir, Target: pgDataInner}},
		Files:       []spec.File{{Path: pgConfPath, Content: conf}, {Path: pgHBAPath, Content: hba}, {Path: patroniPath, Content: cfg}, {Path: patroniInit, Content: init, Mode: "0755"}},
		Ports:       []spec.Port{{Container: pgPort, Host: pgPort}, {Container: patroniPort, Host: patroniPort}},
		Healthcheck: healthcheck("CMD-SHELL", "curl --fail --silent http://127.0.0.1:8008/health >/dev/null && psql -h 127.0.0.1 -U "+pgUser+" -d "+pgDatabase+" -Atqc 'SELECT 1'"),
		Restart:     "always", DependsOn: deps,
	})
	return nil
}

func (c *compilation) patroniConfig(role, m string, hosts []any) (string, error) {
	sync := intParam(c.params(), "sync_replicas", 0)
	overlay := map[string]any{
		"scope": "stroppy-" + c.in.RunID.String(), "name": m, "etcd3_hosts": hosts,
		"restapi_listen": "0.0.0.0:8008", "restapi_connect_address": ip(m) + ":8008",
		"postgresql_listen": "0.0.0.0:5432", "postgresql_connect_address": ip(m) + ":5432",
		"data_dir": pgDataInner + "/pgdata", "bin_dir": "/usr/lib/postgresql/" + c.in.Database.Version + "/bin",
		"superuser_username": pgUser, "superuser_password": pgPassword,
		"replication_username": replUser, "replication_password": replPassword,
		"rewind_username": pgUser, "rewind_password": pgPassword,
		"watchdog_mode": "off", "synchronous_mode": "off", "synchronous_mode_strict": "false",
	}
	if sync > 0 {
		overlay["synchronous_mode"], overlay["synchronous_mode_strict"], overlay["synchronous_node_count"] = "on", "true", sync
	}
	rendered, err := c.render(role, "cfg.patroni.yml@4", "conf", overlay)
	if err != nil {
		return "", err
	}
	// Runtime paths and bootstrap hooks are compiler-owned. Parse the rendered
	// document to preserve schema-backed tuning without duplicate YAML keys.
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(rendered), &doc); err != nil {
		return "", err
	}
	pg, ok := doc["postgresql"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("patroni schema rendered no postgresql mapping")
	}
	delete(pg, "pg_hba")
	pg["custom_conf"] = pgConfPath
	pg["parameters"] = map[string]any{"hba_file": pgHBAPath, "unix_socket_directories": "/var/run/postgresql"}
	boot, ok := doc["bootstrap"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("patroni schema rendered no bootstrap mapping")
	}
	// DCS tuning is cluster-wide. The db role is its canonical config source,
	// even when a db-replica member wins the first bootstrap election.
	if role != topology.RoleDB {
		canonical, err := c.render(topology.RoleDB, "cfg.patroni.yml@4", "conf", overlay)
		if err != nil {
			return "", err
		}
		var cluster struct {
			Bootstrap struct {
				DCS map[string]any `yaml:"dcs"`
			} `yaml:"bootstrap"`
		}
		if err := yaml.Unmarshal([]byte(canonical), &cluster); err != nil {
			return "", err
		}
		boot["dcs"] = cluster.Bootstrap.DCS
	}
	boot["initdb"] = []any{"data-checksums", map[string]any{"locale": strParam(c.params(), "locale", "C")}, map[string]any{"encoding": "UTF8"}}
	boot["post_bootstrap"] = patroniInit
	raw, err := yaml.Marshal(doc)
	return string(raw), err
}
