package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// recipe adds the containers of one database kind to the compilation.
type recipe func(c *compilation) error

// recipes by kind. A kind missing here cannot be launched (the test
// validator reports it).
var recipes = map[catalog.DatabaseKind]recipe{
	catalog.Postgres:   postgresRecipe,
	catalog.OrioleDB:   orioledbRecipe,
	catalog.MySQL:      mysqlRecipe,
	catalog.MariaDB:    mysqlRecipe,
	catalog.Cockroach:  cockroachRecipe,
	catalog.Picodata:   picodataRecipe,
	catalog.YDBManaged: managedYDBRecipe,
	catalog.YDB:        ydbRecipe,
	catalog.PgNoop:     pgNoopRecipe,
	catalog.Noop:       noopRecipe,
	catalog.External:   noopRecipe,
}

// Supported reports whether a kind has a recipe.
func Supported(kind catalog.DatabaseKind) bool {
	_, ok := recipes[kind]
	return ok
}

// imageOf is the catalog image of the database version (or the override).
func (c *compilation) imageOf() (string, error) {
	if c.in.Database.Image != "" {
		return c.in.Database.Image, nil
	}
	kind, ok := c.in.Catalog.Database(c.in.Database.Kind)
	if !ok {
		return "", errs.Newf(errs.CodeInvalid, "unknown database kind %s", c.in.Database.Kind)
	}
	for _, v := range kind.Versions {
		if v.Version == c.in.Database.Version && v.Image != "" {
			return v.Image, nil
		}
	}
	return "", errs.Newf(errs.CodeInvalid, "%s %s has no image", c.in.Database.Kind, c.in.Database.Version)
}

// machinesOf lists the machines of a role.
func (c *compilation) machinesOf(role string) []string { return c.machinesByRole[role] }

// first is the first machine of a role ("" when none).
func (c *compilation) first(role string) string {
	if ms := c.machinesByRole[role]; len(ms) > 0 {
		return ms[0]
	}
	return ""
}

// healthcheck builds a container healthcheck.
func healthcheck(cmd ...string) *spec.Healthcheck {
	return &spec.Healthcheck{Cmd: cmd, Interval: spec.Duration(5e9), Retries: 60}
}

// haproxy renders the proxy container from cfg.haproxy.cfg@2 with a
// listener per (name, port) whose servers are the machines of a role.
type haproxyListener struct {
	Name     string
	BindPort int
	Servers  []haproxyServer
	Check    map[string]any // overrides of the check object; nil = tcp check on the server port
}

type haproxyServer struct {
	Name, Address string
	Port          int
}

func (c *compilation) haproxy(listeners []haproxyListener) error {
	role := topology.RoleProxy
	schemaID := c.schemaFor(role, "cfg.haproxy.cfg@")
	if schemaID == "" {
		return errs.Newf(errs.CodeInvalid, "%s: role %s has no haproxy schema", c.in.Database.Kind, role)
	}
	list := make([]any, 0, len(listeners))
	for _, l := range listeners {
		servers := make([]any, 0, len(l.Servers))
		for _, s := range l.Servers {
			servers = append(servers, map[string]any{"name": s.Name, "address": s.Address, "port": s.Port})
		}
		check := map[string]any{"kind": "tcp", "method": "GET", "uri": "/primary", "expect_status": 200, "port": l.Servers[0].Port, "inter": 3, "fall": 3, "rise": 2}
		for k, v := range l.Check {
			check[k] = v
		}
		item := map[string]any{"name": l.Name, "bind_port": l.BindPort, "mode": "tcp", "balance": "leastconn", "check": check, "servers": servers}
		list = append(list, item)
	}
	cfg, err := c.render(role, schemaID, "conf", map[string]any{"listeners": list, "stats_socket": "/tmp/haproxy.sock"})
	if err != nil {
		return err
	}
	// Native HAProxy metrics use a separate listener from the SQL frontends.
	cfg += "\nlisten stroppy_metrics\n    bind 127.0.0.1:8405\n    mode http\n    http-request use-service prometheus-exporter if { path /metrics }\n"
	for _, m := range c.machinesOf(role) {
		c.add(spec.Container{
			Name: m + "-haproxy", Role: role, Machine: m, Image: imageHAProxy,
			Files:       []spec.File{{Path: "/usr/local/etc/haproxy/haproxy.cfg", Content: cfg}},
			Ports:       portsOf(listeners),
			Healthcheck: healthcheck("CMD", "bash", "-ec", fmt.Sprintf("haproxy -c -f /usr/local/etc/haproxy/haproxy.cfg; exec 3<>/dev/tcp/127.0.0.1/%d", listeners[0].BindPort)),
			Restart:     "always",
			DependsOn:   c.dbContainers(),
		})
		c.out.Spec.Scrapes = append(c.out.Spec.Scrapes, spec.Scrape{Role: role, Job: m + "-haproxy", URL: "http://127.0.0.1:8405/metrics"})
	}
	return nil
}

func portsOf(ls []haproxyListener) []spec.Port {
	out := make([]spec.Port, 0, len(ls))
	for _, l := range ls {
		out = append(out, spec.Port{Container: l.BindPort, Host: l.BindPort})
	}
	return out
}

// dbContainers names the database containers (for depends_on).
func (c *compilation) dbContainers() []string {
	role := topology.RoleDB
	var out []string
	for _, ct := range c.out.Spec.Containers {
		if ct.Role == role && ct.Machine != "" && ct.Name == ct.Machine+"-"+c.engineOf(role) {
			out = append(out, ct.Name)
		}
	}
	return out
}

// engineOf is the container-name suffix of a role's engine (pg_noop →
// pg-noop: names must match ^[a-z][a-z0-9-]*$).
func (c *compilation) engineOf(role string) string {
	for _, n := range c.in.Plan.Nodes {
		if n.Role == role {
			return strings.ReplaceAll(n.Engine, "_", "-")
		}
	}
	return role
}
