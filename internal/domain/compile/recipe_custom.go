package compile

import (
	"fmt"
	"net/url"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// customRecipe runs the tenant's image on every db machine: its env and
// command as given, its port on the host network, its data on the data disk
// when it names a directory, and its own ready check.
func customRecipe(c *compilation) error {
	p := c.params()
	image := c.in.Database.Image
	if image == "" {
		image = strParam(p, "image", "")
	}
	port := intParam(p, "port", pgPort)
	env := map[string]string{}
	if m, ok := p["env"].(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}
	var cmd []string
	if list, ok := p["cmd"].([]any); ok {
		for _, a := range list {
			if s, ok := a.(string); ok {
				cmd = append(cmd, s)
			}
		}
	}
	var check *spec.Healthcheck
	if hc := strParam(p, "healthcheck", ""); hc != "" {
		check = healthcheck("CMD-SHELL", hc)
	}
	dataDir := strParam(p, "data_dir", "")
	for _, m := range c.machinesOf(topology.RoleDB) {
		ct := spec.Container{
			Name: m + "-" + c.engineOf(topology.RoleDB), Role: topology.RoleDB, Machine: m, Image: image,
			Kind:        spec.ContainerKindDatabase,
			Env:         env,
			Cmd:         cmd,
			Ports:       []spec.Port{{Container: port, Host: port}},
			Healthcheck: check,
			Restart:     "always",
		}
		if dataDir != "" {
			ct.Mounts = []spec.Mount{{Source: dataMount + "/custom", Target: dataDir}}
		}
		if mp := intParam(p, "metrics_port", 0); mp > 0 {
			ct.Scrape, ct.ScrapePort = strParam(p, "metrics_path", "/metrics"), mp
		}
		c.add(ct)
		c.customExporter(p, m, ct.Name, port)
	}
	return nil
}

// customExporter adds the exporter of the image's protocol, logged in with
// the user the tenant gave: an image speaking PostgreSQL or MySQL is
// measured like the built-in kinds. Other protocols have only the image's
// own endpoint (metrics_port).
func (c *compilation) customExporter(p map[string]any, machine, dbContainer string, port int) {
	user, password, database := strParam(p, "user", ""), strParam(p, "password", ""), strParam(p, "database", "")
	switch strParam(p, "protocol", "pg") {
	case "pg":
		dsn := (&url.URL{Scheme: "postgresql", User: url.UserPassword(user, password), Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/" + database, RawQuery: "sslmode=disable"}).String()
		c.add(spec.Container{
			Name: machine + "-postgres-exporter", Role: topology.RoleDB, Machine: machine, Image: imagePostgresExport,
			Kind: spec.ContainerKindExporter, Env: map[string]string{"DATA_SOURCE_NAME": dsn}, Cmd: postgresExporterCmd(false),
			Ports:  []spec.Port{{Container: postgresExporterPort, Host: postgresExporterPort}},
			Scrape: "/metrics", Restart: "always", DependsOn: []string{dbContainer},
		})
	case "mysql":
		cmd := mysqldExporterCmd(fmt.Sprintf("127.0.0.1:%d", port), user, false)
		c.add(spec.Container{
			Name: machine + "-mysqld-exporter", Role: topology.RoleDB, Machine: machine, Image: imageMySQLDExporter,
			Kind: spec.ContainerKindExporter, Env: map[string]string{"MYSQLD_EXPORTER_PASSWORD": password}, Cmd: cmd,
			Ports:  []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}},
			Scrape: "/metrics", Restart: "always", DependsOn: []string{dbContainer},
		})
	}
}
